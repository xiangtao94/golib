package env

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// LoadConf 使用Viper按照优先级读取配置：配置文件 > 环境变量 > 默认值
// filename: 配置文件名（不包含扩展名）
// subConf: 子配置目录
// s: 指向结构体的指针，结构体字段需要包含yaml/json/toml等标签
func LoadConf(filename, subConf string, s interface{}) error {
	v := viper.New()

	// 设置配置文件路径和名称
	configPath := filepath.Join(GetConfDirPath(), subConf)
	v.SetConfigName(filename)
	v.SetConfigType("yaml") // 默认使用yaml格式，也支持json、toml等
	v.AddConfigPath(configPath)

	// 设置环境变量前缀（可选）
	v.SetEnvPrefix(GetAppName()) // 使用应用名作为环境变量前缀
	v.AutomaticEnv()             // 自动读取环境变量

	// 支持环境变量中的下划线替换配置键中的点号和下划线
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 尝试读取配置文件
	if err := v.ReadInConfig(); err != nil {
		// 如果配置文件不存在，不报错，继续使用环境变量和默认值
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// 反序列化到结构体
	if err := v.Unmarshal(s); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}

// LoadConfWithDefaults 使用Viper读取配置，支持设置默认值
// filename: 配置文件名（不包含扩展名）
// subConf: 子配置目录
// defaults: 默认值映射（key-value对）
// s: 指向结构体的指针
func LoadConfWithDefaults(filename, subConf string, defaults map[string]interface{}, s interface{}) error {
	v := viper.New()

	// 设置默认值
	for key, value := range defaults {
		v.SetDefault(key, value)
	}

	// 设置配置文件路径和名称
	configPath := filepath.Join(GetConfDirPath(), subConf)
	v.SetConfigName(filename)
	v.SetConfigType("yaml")
	v.AddConfigPath(configPath)

	// 设置环境变量
	v.SetEnvPrefix(GetAppName())
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 手动绑定环境变量（确保所有配置键都能正确映射）
	bindEnvVars(v, defaults)

	// 尝试读取配置文件
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// 反序列化到结构体
	if err := v.Unmarshal(s); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}

// bindEnvVars 手动绑定环境变量
func bindEnvVars(v *viper.Viper, defaults map[string]interface{}) {
	for key := range defaults {
		// 绑定环境变量，Viper会自动处理前缀和键名转换
		v.BindEnv(key)
	}
}

// NewViperInstance 创建一个新的Viper实例，用于更高级的配置管理
// filename: 配置文件名（不包含扩展名）
// subConf: 子配置目录
// configType: 配置文件类型（yaml, json, toml等）
func NewViperInstance(filename, subConf, configType string) *viper.Viper {
	v := viper.New()

	// 设置配置文件路径和名称
	configPath := filepath.Join(GetConfDirPath(), subConf)
	v.SetConfigName(filename)
	v.SetConfigType(configType)
	v.AddConfigPath(configPath)

	// 设置环境变量
	v.SetEnvPrefix(GetAppName())
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	return v
}

// LoadConfFromViper 从已配置的Viper实例加载配置
func LoadConfFromViper(v *viper.Viper, s interface{}) error {
	// 尝试读取配置文件
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// 反序列化到结构体
	if err := v.Unmarshal(s); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}
