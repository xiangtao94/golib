# env

`env` 是 Core module 内的进程环境 package，不依赖 Gin，也不根据 Web framework mode 猜测部署环境。Typed 配置加载由 `pkg/config` 独立拥有。

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

配置文件、defaults、环境变量绑定和启动校验使用 [`pkg/config`](../config/README.md)。`env` 不暴露 Viper，也不提供配置热更新。
