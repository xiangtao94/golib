package env

import (
	"context"
	"os"
	"path/filepath"
	"sync"
)

const DefaultRootPath = "."

const (
	LanguageChinese = "zh"
	LanguageEnglish = "en"
	AppNameEnv      = "APP_NAME"
)

type languageContextKey struct{}

var runtimeState = struct {
	sync.RWMutex
	rootPath string
	appName  string
	language string
}{language: LanguageChinese}

func init() {
	runtimeState.appName = GetEnv(AppNameEnv, "app")
}

// RootPath 返回应用的根目录
func GetRootPath() string {
	runtimeState.RLock()
	defer runtimeState.RUnlock()
	if runtimeState.rootPath != "" {
		return runtimeState.rootPath
	}
	return DefaultRootPath
}

func GetLanguage() string {
	runtimeState.RLock()
	defer runtimeState.RUnlock()
	return runtimeState.language
}

func SetLanguage(lang string) {
	runtimeState.Lock()
	runtimeState.language = lang
	runtimeState.Unlock()
}

func WithLanguage(ctx context.Context, language string) context.Context {
	if ctx == nil {
		panic("env: nil context")
	}
	return context.WithValue(ctx, languageContextKey{}, language)
}

func LanguageFromContext(ctx context.Context) string {
	if ctx != nil {
		if language, ok := ctx.Value(languageContextKey{}).(string); ok && language != "" {
			return language
		}
	}
	return GetLanguage()
}

// GetConfDirPath 返回配置文件目录绝对地址
func GetConfDirPath() string {
	return filepath.Join(GetRootPath(), "conf")
}

// 手动指定SetAppName
func SetAppName(appName string) {
	runtimeState.Lock()
	runtimeState.appName = appName
	runtimeState.Unlock()
}

func GetAppName() string {
	runtimeState.RLock()
	defer runtimeState.RUnlock()
	return runtimeState.appName
}

// SetRootPath 设置应用的根目录
func SetRootPath(r string) {
	runtimeState.Lock()
	runtimeState.rootPath = r
	runtimeState.Unlock()
}

func GetEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
