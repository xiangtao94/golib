# 配置管理 - 基于 Viper

这个包提供了基于 [Viper](https://github.com/spf13/viper) 的配置管理功能，支持多种配置源的优先级读取。

## 功能特性

- ✅ **优先级配置**: 配置文件 > 环境变量 > 默认值
- ✅ **多格式支持**: YAML、JSON、TOML、HCL、Properties等
- ✅ **环境变量自动映射**: 支持嵌套配置的环境变量读取
- ✅ **类型安全**: 自动类型转换和验证
- ✅ **配置热更新**: 支持配置文件变化监听
- ✅ **子配置提取**: 支持提取配置的子树

## 快速开始

### 1. 定义配置结构体

```go
type Config struct {
    Server struct {
        Host string `yaml:"host" json:"host"`
        Port int    `yaml:"port" json:"port"`
    } `yaml:"server" json:"server"`
    
    Database struct {
        Host     string `yaml:"host" json:"host"`
        Port     int    `yaml:"port" json:"port"`
        Username string `yaml:"username" json:"username"`
        Password string `yaml:"password" json:"password"`
        Name     string `yaml:"name" json:"name"`
    } `yaml:"database" json:"database"`
    
    Debug    bool     `yaml:"debug" json:"debug"`
    LogLevel string   `yaml:"log_level" json:"log_level"`
    Tags     []string `yaml:"tags" json:"tags"`
}
```

### 2. 使用配置

```go
package main

import (
    "log"
    "your-project/pkg/env"
)

func main() {
    // 设置应用名称（用于环境变量前缀）
    env.SetAppName("myapp")
    
    var config Config
    
    // 方法1: 简单加载配置
    err := env.LoadConf("app", "production", &config)
    if err != nil {
        log.Fatal(err)
    }
    
    // 方法2: 带默认值的配置加载
    defaults := map[string]interface{}{
        "server.host":     "localhost",
        "server.port":     8080,
        "database.host":   "localhost",
        "database.port":   3306,
        "debug":           false,
        "log_level":       "info",
    }
    
    err = env.LoadConfWithDefaults("app", "production", defaults, &config)
    if err != nil {
        log.Fatal(err)
    }
    
    // 使用配置
    fmt.Printf("服务器地址: %s:%d\n", config.Server.Host, config.Server.Port)
    fmt.Printf("数据库: %s@%s:%d/%s\n", 
               config.Database.Username, 
               config.Database.Host, 
               config.Database.Port, 
               config.Database.Name)
}
```

## 配置优先级

配置读取遵循以下优先级顺序（从高到低）：

1. **配置文件** - `conf/{subConf}/{filename}.yaml`
2. **环境变量** - `{APP_NAME}_{CONFIG_KEY}`
3. **默认值** - 代码中设置的默认值

### 环境变量映射规则

- 自动添加应用名称前缀：`{APP_NAME}_`
- 嵌套配置用下划线分隔：`server.host` → `MYAPP_SERVER_HOST`
- 配置键中的点号替换为下划线：`log_level` → `MYAPP_LOG_LEVEL`

## 使用示例

### 示例配置文件

创建 `conf/production/app.yaml`:

```yaml
server:
  host: "0.0.0.0"
  port: 8080

database:
  host: "db.example.com"
  port: 3306
  username: "appuser"
  password: "${DB_PASSWORD}"  # 支持环境变量替换
  name: "production_db"

debug: false
log_level: "info"
tags:
  - "production"
  - "web"
```

### 环境变量示例

```bash
export MYAPP_SERVER_HOST="192.168.1.100"
export MYAPP_SERVER_PORT="9000"
export MYAPP_DEBUG="true"
export MYAPP_LOG_LEVEL="debug"
export DB_PASSWORD="secret123"
```

### 高级用法

```go
// 创建自定义Viper实例
v := env.NewViperInstance("app", "production", "yaml")

// 设置默认值
v.SetDefault("server.timeout", 30)
v.SetDefault("server.max_connections", 1000)

// 配置监听（可选）
v.WatchConfig()
v.OnConfigChange(func(e fsnotify.Event) {
    fmt.Println("配置文件已更新:", e.Name)
    // 重新加载配置
    env.LoadConfFromViper(v, &config)
})

// 加载配置
err := env.LoadConfFromViper(v, &config)
if err != nil {
    log.Fatal(err)
}

// 获取子配置
dbConfig := v.Sub("database")
if dbConfig != nil {
    var db DatabaseConfig
    dbConfig.Unmarshal(&db)
}
```

## API 参考

### LoadConf

```go
func LoadConf(filename, subConf string, s interface{}) error
```

基本的配置加载函数。

- `filename`: 配置文件名（不包含扩展名）
- `subConf`: 子配置目录
- `s`: 指向配置结构体的指针

### LoadConfWithDefaults

```go
func LoadConfWithDefaults(filename, subConf string, defaults map[string]interface{}, s interface{}) error
```

带默认值的配置加载函数。

### NewViperInstance

```go
func NewViperInstance(filename, subConf, configType string) *viper.Viper
```

创建自定义Viper实例，用于高级配置管理。

### LoadConfFromViper

```go
func LoadConfFromViper(v *viper.Viper, s interface{}) error
```

从已配置的Viper实例加载配置。

## 支持的配置格式

- **YAML** (推荐)
- **JSON**
- **TOML**
- **HCL**
- **Properties**
- **Envfile**

## 错误处理

所有函数都返回 `error`，确保在生产环境中正确处理：

```go
if err := env.LoadConf("app", "production", &config); err != nil {
    log.Fatalf("加载配置失败: %v", err)
}
```

## 最佳实践

1. **使用结构体标签**: 为不同格式添加相应标签
2. **设置默认值**: 确保应用在任何环境下都能正常运行
3. **环境变量命名**: 使用清晰的命名约定
4. **敏感信息**: 通过环境变量传递密码等敏感信息
5. **配置验证**: 在加载后验证配置的有效性

```go
// 配置验证示例
func (c *Config) Validate() error {
    if c.Server.Port <= 0 || c.Server.Port > 65535 {
        return fmt.Errorf("无效的端口号: %d", c.Server.Port)
    }
    if c.Database.Host == "" {
        return fmt.Errorf("数据库主机不能为空")
    }
    return nil
}

// 使用
var config Config
if err := env.LoadConf("app", "production", &config); err != nil {
    log.Fatal(err)
}
if err := config.Validate(); err != nil {
    log.Fatal("配置验证失败:", err)
}
``` 