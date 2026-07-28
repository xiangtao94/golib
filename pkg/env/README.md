# env

`env` 是独立配置 module，不依赖 Gin，也不根据 Web framework mode 猜测部署环境。

## 应用设置

```go
env.SetAppName("users")
env.SetRootPath("/srv/users")
env.SetLanguage(env.LanguageChinese)
```

应用名初始读取 `APP_NAME`，缺省为 `app`。`SetRootPath` 始终按调用方要求设置，不包含 Docker/release 隐式分支。

请求级语言使用标准 context：

```go
ctx := env.WithLanguage(context.Background(), env.LanguageEnglish)
language := env.LanguageFromContext(ctx)
```

## 配置

推荐为所有配置项提供默认值，以便 Viper 能稳定绑定对应环境变量：

```go
defaults := map[string]any{
	"server.host": "127.0.0.1",
	"server.port": 8080,
}

var config Config
if err := env.LoadConfWithDefaults("app", "production", defaults, &config); err != nil {
	return err
}
```

配置文件路径为 `<root>/conf/<subConf>/<filename>.yaml`。优先级是环境变量 > 配置文件 > 默认值，环境变量格式为 `<APP_NAME>_<KEY>`，嵌套点号转换为下划线，例如 `USERS_SERVER_PORT`。

缺少配置文件不是 error；格式错误和反序列化错误会返回 error。package 不提供 panic 型加载或隐式热更新。需要监听时，由业务通过 `NewViperInstance` 明确持有 Viper 生命周期。
