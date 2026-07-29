// Package config loads immutable, typed application configuration.
package config

import (
	"encoding"
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

var textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()

type Options struct {
	// File is an optional exact config file path. When set, the file must
	// exist unless OptionalFile is explicitly enabled.
	File string
	// Type is used when File has no extension. It defaults to yaml.
	Type string
	// EnvPrefix is prepended to environment keys derived from the target type.
	EnvPrefix string
	// OptionalFile allows a declared File to be absent.
	OptionalFile bool
	// Defaults use dotted mapstructure paths, for example "database.timeout".
	Defaults map[string]any
}

type ValidateFunc[T any] func(T) error

// Load resolves defaults, then File, then environment variables into T.
// Unknown file keys are rejected and validation runs after decoding.
func Load[T any](options Options, validate ValidateFunc[T]) (T, error) {
	var result T
	keys, err := configKeys(reflect.TypeOf(result), "")
	if err != nil {
		return result, err
	}

	instance := viper.New()
	instance.SetConfigType(configType(options))
	instance.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	if options.EnvPrefix != "" {
		instance.SetEnvPrefix(options.EnvPrefix)
	}
	instance.AutomaticEnv()

	for key, value := range options.Defaults {
		instance.SetDefault(key, value)
	}
	for _, key := range keys {
		if err := instance.BindEnv(key); err != nil {
			return result, fmt.Errorf("bind config environment %q: %w", key, err)
		}
	}

	if options.File != "" {
		instance.SetConfigFile(options.File)
		if err := instance.ReadInConfig(); err != nil {
			if !options.OptionalFile || !isMissingFile(err) {
				return result, fmt.Errorf("read config %q: %w", options.File, err)
			}
		}
	}

	if err := instance.UnmarshalExact(
		&result,
		viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
			mapstructure.TextUnmarshallerHookFunc(),
		)),
	); err != nil {
		return result, fmt.Errorf("decode config: %w", err)
	}
	if validate != nil {
		if err := validate(result); err != nil {
			return result, fmt.Errorf("validate config: %w", err)
		}
	}
	return result, nil
}

func configType(options Options) string {
	if options.Type != "" {
		return options.Type
	}
	return "yaml"
}

func isMissingFile(err error) bool {
	_, notFound := errors.AsType[viper.ConfigFileNotFoundError](err)
	return notFound || errors.Is(err, fs.ErrNotExist)
}

func configKeys(valueType reflect.Type, prefix string) ([]string, error) {
	for valueType != nil && valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
	}
	if valueType == nil || valueType.Kind() != reflect.Struct {
		return nil, errors.New("config target must be a struct")
	}

	var keys []string
	for field := range valueType.Fields() {
		if !field.IsExported() {
			continue
		}
		name, squash, skip := fieldName(field)
		if skip {
			continue
		}

		fieldPrefix := prefix
		if !squash {
			fieldPrefix = joinKey(prefix, name)
		}
		fieldType := field.Type
		for fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}
		if fieldType.Kind() == reflect.Struct && !isTextScalar(fieldType) {
			nested, err := configKeys(fieldType, fieldPrefix)
			if err != nil {
				return nil, err
			}
			keys = append(keys, nested...)
			continue
		}
		keys = append(keys, fieldPrefix)
	}
	return keys, nil
}

func isTextScalar(valueType reflect.Type) bool {
	return valueType.Implements(textUnmarshalerType) ||
		reflect.PointerTo(valueType).Implements(textUnmarshalerType)
}

func fieldName(field reflect.StructField) (name string, squash bool, skip bool) {
	tag := field.Tag.Get("mapstructure")
	if tag == "" {
		tag = field.Tag.Get("yaml")
	}
	parts := strings.Split(tag, ",")
	switch parts[0] {
	case "-":
		return "", false, true
	case "":
		name = strings.ToLower(field.Name)
	default:
		name = parts[0]
	}
	for _, option := range parts[1:] {
		if option == "squash" {
			squash = true
		}
	}
	return name, squash, false
}

func joinKey(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}
