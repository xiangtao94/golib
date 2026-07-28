# health

`Gate` 提供框架无关的 liveness/readiness `http.Handler`：

- 新建时 readiness 为 false；
- 启动完成后由应用调用 `SetReady`；
- draining 开始前调用 `SetNotReady`；
- liveness 不检查数据库或外部依赖，避免依赖故障触发进程级联重启。

具体依赖是否影响 readiness 由应用决定，基础库不自动探测业务连接。

