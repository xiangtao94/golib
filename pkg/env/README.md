# env

`env` 是 Core module 内的进程运行状态 package，不依赖 Gin，也不根据 Web framework mode 猜测部署环境。应用配置应先通过 `pkg/config` 加载和校验，再将应用名等启动参数显式传给运行组件；不要同时用 `config.Load` 和 `env.GetEnv` 加载同一配置项。

## 应用设置

```go
env.SetAppName("users")
env.SetRootPath("/srv/users")
env.SetLanguage(env.LanguageChinese)
```

Gin 服务在启动时将应用名作为 `golib.NewApp(appName, ...)` 或 `golib.Bootstraps(engine, appName, ...)` 的必填参数，直接使用 `env` 的非 Gin 程序仍可调用 `SetAppName`。应用名初始读取 `APP_NAME`，缺省为 `app`。`SetRootPath` 始终按调用方要求设置，不包含 Docker/release 隐式分支。

请求级语言使用标准 context：

```go
ctx := env.WithLanguage(context.Background(), env.LanguageEnglish)
language := env.LanguageFromContext(ctx)
```

配置文件、defaults、环境变量绑定和启动校验使用 [`pkg/config`](../config/README.md)。`env` 不暴露 Viper，也不提供配置热更新。
