# Redis 客户端

基于 `go-redis/v9` 封装的 Redis 客户端，提供完整的日志记录、连接池管理和高性能缓存功能。

## 功能特性

- ✅ **Universal Client**: 支持单机、集群、哨兵模式
- ✅ **连接池管理**: 优化的连接池配置和管理
- ✅ **完整日志**: 集成 zlog 记录所有 Redis 操作
- ✅ **超时控制**: 可配置的连接、读写超时时间
- ✅ **重试机制**: 内置的请求重试策略

## 快速开始

### 1. 配置结构体

```go
type RedisConf struct {
    Addr            string        `yaml:"addr"`            // Redis 地址，集群模式用逗号分隔
    Db              int           `yaml:"db"`              // 数据库编号
    Password        string        `yaml:"password"`        // 密码
    MaxIdle         int           `yaml:"maxIdle"`         // 最小空闲连接数
    MaxActive       int           `yaml:"maxActive"`       // 最大连接数
    IdleTimeout     time.Duration `yaml:"idleTimeout"`     // 连接空闲超时时间
    MaxConnLifetime time.Duration `yaml:"maxConnLifetime"` // 连接最大生存时间
    ConnTimeOut     time.Duration `yaml:"connTimeOut"`     // 连接超时时间
    ReadTimeOut     time.Duration `yaml:"readTimeOut"`     // 读超时时间
    WriteTimeOut    time.Duration `yaml:"writeTimeOut"`    // 写超时时间
    MaxRetries      int           `yaml:"maxRetries"`      // 最大重试次数
}
```

### 2. 初始化客户端

```go
package main

import (
    "time"
    "github.com/xiangtao94/golib/pkg/redis"
)

func main() {
    conf := redis.RedisConf{
        Addr:            "localhost:6379",
        Db:              0,
        Password:        "",
        MaxIdle:         10,
        MaxActive:       50,
        IdleTimeout:     5 * time.Minute,
        MaxConnLifetime: 30 * time.Minute,
        ConnTimeOut:     3 * time.Second,
        ReadTimeOut:     2 * time.Second,
        WriteTimeOut:    2 * time.Second,
        MaxRetries:      3,
    }
    
    client, err := redis.InitRedisClient(conf)
    if err != nil {
        log.Fatal(err)
    }
}
```

## 使用示例

### 基本操作

```go
ctx := context.Background()

// 字符串操作
err := client.Set(ctx, "key", "value", time.Hour).Err()
if err != nil {
    log.Fatal(err)
}

val, err := client.Get(ctx, "key").Result()
if err != nil {
    log.Fatal(err)
}
fmt.Println("value:", val)

// 数值操作
err = client.Set(ctx, "counter", 0, 0).Err()
newVal, err := client.Incr(ctx, "counter").Result()
fmt.Println("counter:", newVal)
```

### 哈希操作

```go
// 设置哈希字段
err := client.HSet(ctx, "user:1001", "name", "张三", "age", 30).Err()
if err != nil {
    log.Fatal(err)
}

// 获取哈希字段
name, err := client.HGet(ctx, "user:1001", "name").Result()
if err != nil {
    log.Fatal(err)
}

// 获取所有哈希字段
userInfo, err := client.HGetAll(ctx, "user:1001").Result()
for field, value := range userInfo {
    fmt.Printf("%s: %s\n", field, value)
}
```

### 列表操作

```go
// 向列表添加元素
err := client.LPush(ctx, "queue", "task1", "task2", "task3").Err()
if err != nil {
    log.Fatal(err)
}

// 从列表获取元素
task, err := client.RPop(ctx, "queue").Result()
if err != nil {
    log.Fatal(err)
}
fmt.Println("task:", task)

// 获取列表长度
length, err := client.LLen(ctx, "queue").Result()
fmt.Println("queue length:", length)
```

### 集合操作

```go
// 向集合添加成员
err := client.SAdd(ctx, "tags", "go", "redis", "cache").Err()
if err != nil {
    log.Fatal(err)
}

// 检查成员是否存在
exists, err := client.SIsMember(ctx, "tags", "go").Result()
fmt.Println("go exists:", exists)

// 获取所有成员
members, err := client.SMembers(ctx, "tags").Result()
fmt.Println("tags:", members)
```

### 管道操作

```go
// 使用管道批量操作
pipe := client.Pipeline()
pipe.Set(ctx, "key1", "value1", time.Hour)
pipe.Set(ctx, "key2", "value2", time.Hour)
pipe.Incr(ctx, "counter")

cmds, err := pipe.Exec(ctx)
if err != nil {
    log.Fatal(err)
}

// 处理结果
for _, cmd := range cmds {
    fmt.Println(cmd.String())
}
```

### 事务操作

```go
// 使用事务
err := client.Watch(ctx, func(tx *redis.Tx) error {
    // 在事务内执行操作
    _, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
        pipe.Set(ctx, "key1", "value1", time.Hour)
        pipe.Set(ctx, "key2", "value2", time.Hour)
        return nil
    })
    return err
}, "watched_key")

if err != nil {
    log.Fatal(err)
}
```

## 集群配置

```go
// Redis 集群配置
conf := redis.RedisConf{
    Addr: "192.168.1.1:6379,192.168.1.2:6379,192.168.1.3:6379", // 集群节点地址
    Password: "cluster-password",
    MaxIdle:  10,
    MaxActive: 50,
    // 其他配置...
}

client, err := redis.InitRedisClient(conf)
```

## 配置默认值

如果配置项为空，系统会使用以下默认值：

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| MaxIdle | 10 | 最小空闲连接数 |
| MaxActive | 50 | 最大连接数 |
| IdleTimeout | 5分钟 | 连接空闲超时时间 |
| MaxConnLifetime | 30分钟 | 连接最大生存时间 |
| ConnTimeOut | 3秒 | 连接超时时间 |
| ReadTimeOut | 2秒 | 读超时时间 |
| WriteTimeOut | 2秒 | 写超时时间 |
| MaxRetries | 3 | 最大重试次数 |

## 日志记录

启用 DEBUG 日志时，客户端记录 Redis 操作元数据：

- Redis 命令名和参数数量，不记录参数值
- 执行时间
- 错误信息（如果有）
- 请求ID（集成 Gin 框架）

```go
// 日志示例输出
// {"level":"DEBUG","time":"2024-01-01 12:00:00.123","msg":"redis","command":"set","argumentCount":5,"cost":"1.2ms","requestId":"req-123"}
```

DEBUG 未启用时不会格式化命令或构造日志字段。

## 完整示例

```go
package main

import (
    "context"
    "log"
    "time"
    
    "github.com/gin-gonic/gin"
    "github.com/xiangtao94/golib/pkg/redis"
    "github.com/xiangtao94/golib/pkg/env"
)

func main() {
    // 配置Redis客户端
    conf := redis.RedisConf{
        Addr:        "localhost:6379",
        Password:    "",
        MaxActive:   100,
        MaxIdle:     20,
        ConnTimeOut: 5 * time.Second,
    }
    
    client, err := redis.InitRedisClient(conf)
    if err != nil {
        log.Fatal(err)
    }
    
    r := gin.Default()
    
    // 缓存用户信息
    r.GET("/user/:id", func(c *gin.Context) {
        userID := c.Param("id")
        cacheKey := "webapp:user:" + userID
        
        // 尝试从缓存获取
        cached, err := client.Get(c, cacheKey).Result()
        if err == nil {
            c.JSON(200, gin.H{"data": cached, "from": "cache"})
            return
        }
        
        // 模拟从数据库获取
        userData := map[string]interface{}{
            "id":   userID,
            "name": "用户" + userID,
        }
        
        // 存入缓存
        client.Set(c, cacheKey, userData, time.Hour)
        
        c.JSON(200, gin.H{"data": userData, "from": "database"})
    })
    
    r.Run(":8080")
}
```

## 注意事项

- 客户端支持单机、集群、哨兵等多种部署模式
- 键名命名空间属于业务约定，应由调用方显式构造
- 连接池会自动管理连接的创建和回收
- DEBUG 模式记录命令元数据，不复制或暴露完整参数
- 重试机制会自动处理网络抖动等临时故障
