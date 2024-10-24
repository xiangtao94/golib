package redis

import (
	"github.com/tiant-go/golib/pkg/zlog"
	"time"

	redigo "github.com/gomodule/redigo/redis"
)

func (r *Redis) Lua(script string, keyCount int, keysAndArgs ...interface{}) (interface{}, error) {
	start := time.Now()

	lua := redigo.NewScript(keyCount, script)

	conn := r.pool.Get()
	if err := conn.Err(); err != nil {
		r.logger.Error("get connection error: "+err.Error(), r.commonFields(r.ctx)...)
		return nil, err
	}
	defer conn.Close()

	reply, err := lua.Do(conn, keysAndArgs...)

	ralCode := 0
	msg := "pipeline exec succ"
	if err != nil {
		ralCode = -1
		msg = "pipeline exec error: " + err.Error()
	}
	fields := append(r.commonFields(r.ctx),
		zlog.Int("ralCode", ralCode),
	)
	fields = append(fields, zlog.AppendCostTime(start, time.Now())...)
	r.logger.Info(msg, fields...)

	return reply, err
}
