# lifecycle

`Manager` 顺序执行 `OnStart`，启动失败时回滚已经成功启动的 hook，并在关闭时反序执行全部 `OnStop`。

调用方持有 OS signal 和 stop deadline；package 不捕获 signal。`Stop` 幂等并合并所有关闭错误。Hook 必须有稳定名称，且至少包含一个回调。

