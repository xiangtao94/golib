# config

`config.Load[T]` 按 defaults → 文件 → 环境变量的顺序加载不可变 typed snapshot，并在返回前执行调用方校验。

```go
type Config struct {
	HTTP struct {
		Addr string `mapstructure:"addr"`
	} `mapstructure:"http"`
}

loaded, err := config.Load[Config](config.Options{
	File:      "/etc/app/config.yaml",
	EnvPrefix: "APP",
	Defaults:  map[string]any{"http.addr": ":8080"},
}, func(value Config) error {
	if value.HTTP.Addr == "" {
		return errors.New("http.addr is required")
	}
	return nil
})
```

声明 `File` 后文件默认必须存在；只有显式设置 `OptionalFile` 才允许缺失。未知配置字段会被拒绝，错误不会输出配置值。`time.Duration`、字符串 slice，以及实现 `encoding.TextUnmarshaler` 的 scalar（如 `time.Time`）可从环境变量解码。package 不暴露 Viper，也不提供隐式热更新。
