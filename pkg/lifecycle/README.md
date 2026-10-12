# lifecycle

`Manager` 顺序执行 `OnStart`，启动失败时回滚已经成功启动的 hook，并在关闭时反序执行全部 `OnStop`。

调用方持有 OS signal 和 stop deadline；package 不捕获 signal。`Stop` 幂等并合并所有关闭错误。Hook 必须有稳定名称，且至少包含一个回调。

已初始化资源可在 `Start` 前或之后通过 `AddStop(name, closeFunc)` 登记；关闭时按登记顺序的反序执行。根包 `NewApp` 和 `Bootstraps` 使用这个 Manager 管理日志及应用注册的关闭回调。
