package env

import (
	"os"
	"testing"
)

// 示例配置结构体
type ExampleConfig struct {
	// 服务器配置
	Server struct {
		Host string `yaml:"host" json:"host" toml:"host" mapstructure:"host"`
		Port int    `yaml:"port" json:"port" toml:"port" mapstructure:"port"`
	} `yaml:"server" json:"server" toml:"server" mapstructure:"server"`

	// 数据库配置
	Database struct {
		Host     string `yaml:"host" json:"host" toml:"host" mapstructure:"host"`
		Port     int    `yaml:"port" json:"port" toml:"port" mapstructure:"port"`
		Username string `yaml:"username" json:"username" toml:"username" mapstructure:"username"`
		Password string `yaml:"password" json:"password" toml:"password" mapstructure:"password"`
		Name     string `yaml:"name" json:"name" toml:"name" mapstructure:"name"`
	} `yaml:"database" json:"database" toml:"database" mapstructure:"database"`

	// 其他配置
	Debug    bool     `yaml:"debug" json:"debug" toml:"debug" mapstructure:"debug"`
	Timeout  int      `yaml:"timeout" json:"timeout" toml:"timeout" mapstructure:"timeout"`
	Tags     []string `yaml:"tags" json:"tags" toml:"tags" mapstructure:"tags"`
	LogLevel string   `yaml:"log_level" json:"log_level" toml:"log_level" mapstructure:"log_level"`
	MaxConns int      `yaml:"max_conns" json:"max_conns" toml:"max_conns" mapstructure:"max_conns"`
}

// TestLoadConf 测试使用Viper的配置读取功能
func TestLoadConf(t *testing.T) {
	// 设置应用名称用于环境变量前缀
	SetAppName("myapp")

	// 清除环境变量
	os.Unsetenv("MYAPP_SERVER_HOST")
	os.Unsetenv("MYAPP_SERVER_PORT")
	os.Unsetenv("MYAPP_DATABASE_HOST")
	os.Unsetenv("MYAPP_DEBUG")
	os.Unsetenv("MYAPP_LOG_LEVEL")

	var config ExampleConfig

	// 测试基本配置读取（使用不存在的配置文件）
	t.Run("BasicConfigLoad", func(t *testing.T) {
		err := LoadConf("nonexistent", "", &config)
		if err != nil {
			t.Errorf("LoadConf should not return error for missing config file: %v", err)
		}

		// 由于没有配置文件和环境变量，结构体应该是零值
		if config.Server.Host != "" {
			t.Errorf("Expected Server.Host to be empty, got '%s'", config.Server.Host)
		}
	})

	// 测试带默认值的配置读取
	t.Run("ConfigWithDefaults", func(t *testing.T) {
		defaults := map[string]interface{}{
			"server.host":       "localhost",
			"server.port":       8080,
			"database.host":     "localhost",
			"database.port":     3306,
			"database.username": "root",
			"database.name":     "test",
			"debug":             false,
			"timeout":           30,
			"tags":              []string{"dev", "test"},
			"log_level":         "info",
			"max_conns":         100,
		}

		err := LoadConfWithDefaults("nonexistent", "", defaults, &config)
		if err != nil {
			t.Errorf("LoadConfWithDefaults failed: %v", err)
		}

		// 验证默认值
		if config.Server.Host != "localhost" {
			t.Errorf("Expected Server.Host to be 'localhost', got '%s'", config.Server.Host)
		}
		if config.Server.Port != 8080 {
			t.Errorf("Expected Server.Port to be 8080, got %d", config.Server.Port)
		}
		if config.Database.Port != 3306 {
			t.Errorf("Expected Database.Port to be 3306, got %d", config.Database.Port)
		}
		if config.Debug != false {
			t.Errorf("Expected Debug to be false, got %v", config.Debug)
		}
		if config.LogLevel != "info" {
			t.Errorf("Expected LogLevel to be 'info', got '%s'", config.LogLevel)
		}
		if len(config.Tags) != 2 || config.Tags[0] != "dev" || config.Tags[1] != "test" {
			t.Errorf("Expected Tags to be ['dev', 'test'], got %v", config.Tags)
		}
	})

	// 测试环境变量覆盖默认值
	t.Run("EnvironmentVariableOverride", func(t *testing.T) {
		// 设置环境变量
		os.Setenv("MYAPP_SERVER_HOST", "0.0.0.0")
		os.Setenv("MYAPP_SERVER_PORT", "9000")
		os.Setenv("MYAPP_DEBUG", "true")
		os.Setenv("MYAPP_LOG_LEVEL", "debug")
		os.Setenv("MYAPP_MAX_CONNS", "200")

		defaults := map[string]interface{}{
			"server.host": "localhost",
			"server.port": 8080,
			"debug":       false,
			"log_level":   "info",
			"max_conns":   100,
		}

		err := LoadConfWithDefaults("nonexistent", "", defaults, &config)
		if err != nil {
			t.Errorf("LoadConfWithDefaults failed: %v", err)
		}

		// 验证环境变量覆盖了默认值
		if config.Server.Host != "0.0.0.0" {
			t.Errorf("Expected Server.Host to be '0.0.0.0', got '%s'", config.Server.Host)
		}
		if config.Server.Port != 9000 {
			t.Errorf("Expected Server.Port to be 9000, got %d", config.Server.Port)
		}
		if config.Debug != true {
			t.Errorf("Expected Debug to be true, got %v", config.Debug)
		}
		if config.LogLevel != "debug" {
			t.Errorf("Expected LogLevel to be 'debug', got '%s'", config.LogLevel)
		}
		if config.MaxConns != 200 {
			t.Errorf("Expected MaxConns to be 200, got %d", config.MaxConns)
		}

		// 清理环境变量
		os.Unsetenv("MYAPP_SERVER_HOST")
		os.Unsetenv("MYAPP_SERVER_PORT")
		os.Unsetenv("MYAPP_DEBUG")
		os.Unsetenv("MYAPP_LOG_LEVEL")
		os.Unsetenv("MYAPP_MAX_CONNS")
	})

	// 测试Viper实例创建
	t.Run("ViperInstance", func(t *testing.T) {
		v := NewViperInstance("app", "dev", "yaml")
		if v == nil {
			t.Error("NewViperInstance should not return nil")
		}

		// 设置一些默认值
		v.SetDefault("test.value", "default")

		// 使用Viper实例加载配置
		var testConfig struct {
			Test struct {
				Value string `yaml:"value"`
			} `yaml:"test"`
		}

		err := LoadConfFromViper(v, &testConfig)
		if err != nil {
			t.Errorf("LoadConfFromViper failed: %v", err)
		}

		if testConfig.Test.Value != "default" {
			t.Errorf("Expected Test.Value to be 'default', got '%s'", testConfig.Test.Value)
		}
	})
}

// 使用示例
func ExampleLoadConf() {
	// 设置应用名称
	SetAppName("myapp")

	// 定义配置结构体
	var config ExampleConfig

	// 方法1：简单加载配置
	err := LoadConf("app", "production", &config)
	if err != nil {
		panic(err)
	}

	// 方法2：带默认值的配置加载
	defaults := map[string]interface{}{
		"server.host": "localhost",
		"server.port": 8080,
		"debug":       false,
	}

	err = LoadConfWithDefaults("app", "production", defaults, &config)
	if err != nil {
		panic(err)
	}

	// 方法3：使用自定义Viper实例
	v := NewViperInstance("app", "production", "yaml")
	v.SetDefault("server.host", "localhost")
	v.SetDefault("server.port", 8080)

	err = LoadConfFromViper(v, &config)
	if err != nil {
		panic(err)
	}

	// 使用配置
	_ = config.Server.Host // 来自配置文件、环境变量或默认值
	_ = config.Server.Port // 来自配置文件、环境变量或默认值
}
