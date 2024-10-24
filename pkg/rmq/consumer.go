package rmq

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/tiant-go/golib/pkg/env"
	"github.com/tiant-go/golib/pkg/zlog"
	"log"
	"net/http/httptest"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

type pushConsumer struct {
	conf     ConsumerConf
	consumer rocketmq.PushConsumer
	engine   *gin.Engine
}

func newPushConsumer(conf ConsumerConf) (*pushConsumer, error) {
	instance := instancePrefix + conf.Service
	var c consumer.MessageModel
	if conf.Broadcast {
		instance = instance + "-consumer"
		c = consumer.BroadCasting
	} else {
		instance = instance + "-" + env.LocalIP + "-" + strconv.Itoa(os.Getpid()) + "-consumer"
		c = consumer.Clustering
	}

	nameServerAddr := getHostListByDns([]string{conf.NameServer})
	options := []consumer.Option{
		consumer.WithInstance(instance),
		consumer.WithGroupName(conf.Group),
		consumer.WithAutoCommit(true),
		consumer.WithNsResolver(primitive.NewPassthroughResolver(nameServerAddr)),
		consumer.WithConsumerOrder(conf.Orderly),
		consumer.WithConsumeMessageBatchMaxSize(conf.Batch),
		consumer.WithMaxReconsumeTimes(int32(conf.Retry)),
		consumer.WithStrategy(consumer.AllocateByAveragely),
		consumer.WithConsumeFromWhere(consumer.ConsumeFromLastOffset),
		consumer.WithConsumerModel(c),
		consumer.WithSuspendCurrentQueueTimeMillis(conf.RetryInterval), //顺序消费设置 失败重试间隔
	}
	// 配置开启了消息轨迹
	if conf.Trace {
		options = append(options, consumer.WithTrace(&primitive.TraceConfig{
			TraceTopic: conf.TraceTopic,
			Access:     primitive.Local,
			Resolver:   primitive.NewPassthroughResolver(nameServerAddr),
		}))
	}

	if conf.Auth.AccessKey != "" && conf.Auth.SecretKey != "" {
		options = append(options, consumer.WithCredentials(primitive.Credentials{
			AccessKey: conf.Auth.AccessKey,
			SecretKey: conf.Auth.SecretKey,
		}))
	}

	con, err := rocketmq.NewPushConsumer(options...)
	if err != nil {
		logger.Error("failed to create consumer", fields(zlog.String("error", err.Error()))...)
		return nil, err
	}

	return &pushConsumer{
		conf:     conf,
		consumer: con,
	}, nil
}

func (c *pushConsumer) start(callback MessageCallback) (err error) {
	err = c.subscribe(callback)
	if err != nil {
		return err
	}

	return c.consumer.Start()
}

func (c *pushConsumer) stop() error {
	return c.consumer.Shutdown()
}

func (c *pushConsumer) subscribe(callback MessageCallback) (err error) {
	if callback == nil {
		return errors.Wrap(ErrRmqSvcInvalidOperation, "nil callback")
	}

	cb := func(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
		for _, m := range msgs {
			if ctx.Err() != nil {
				logger.Error("stop consume cause ctx cancelled", fields(
					zlog.String("service", c.conf.Service),
					zlog.String("error", err.Error()))...)
				return consumer.SuspendCurrentQueueAMoment, ctx.Err()
			}
			if err := c.call(callback, m); err != nil {
				return consumer.SuspendCurrentQueueAMoment, nil
			}
		}
		return consumer.ConsumeSuccess, nil
	}

	var expr string
	if len(c.conf.Tags) == 0 {
		expr = "*"
	} else if len(c.conf.Tags) == 1 {
		expr = c.conf.Tags[0]
	} else {
		expr = strings.Join(c.conf.Tags, "||")
	}

	selector := consumer.MessageSelector{
		Type:       consumer.TAG,
		Expression: expr,
	}

	err = c.consumer.Subscribe(c.conf.Topic, selector, cb)
	if err != nil {
		logger.Error("failed to subscribe", fields(zlog.String("error", err.Error()))...)
		return err
	}

	return nil
}

func (c *pushConsumer) call(fn MessageCallback, m *primitive.MessageExt) (err error) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	start := time.Now()

	defer func() {
		if r := recover(); r != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]

			info, _ := json.Marshal(map[string]interface{}{
				"time":   time.Now().Format("2006-01-02 15:04:05"),
				"level":  "error",
				"module": "stack",
			})
			logger.Error("failed to consume message: " + fmt.Sprintf("%v", r))
			log.Printf("%s\n-------------------stack-start-------------------\n%v\n%s\n-------------------stack-end-------------------\n", string(info), r, buf)
		}
	}()

	produceTime := time.Unix(m.BornTimestamp/1000, m.BornTimestamp%1000*int64(time.Millisecond))
	mw := &messageWrapper{
		msg:   &m.Message,
		msgID: m.MsgId,
		time:  produceTime,
		retry: int(m.ReconsumeTimes),
	}

	err = fn(ctx, mw)
	consumeResult := "consume message success"
	ralCode := 0
	if err != nil {
		ralCode = -1
		consumeResult = err.Error()
	}

	// 用户自定义notice
	end := time.Now()
	var fields []zlog.Field

	// not modifiy original msg body
	var bodyLog bytes.Buffer
	if len(m.Body) > 100 {
		bodyLog.Write(m.Body[0:100])
		bodyLog.WriteString("...")
	} else {
		bodyLog.Write(m.Body)
	}
	fields = append(fields,
		zlog.String("service", c.conf.Service),
		zlog.String("addr", c.conf.NameServer),
		zlog.String("method", "consume"),
		zlog.String("topic", m.Topic),
		zlog.String("message", bodyLog.String()),
		zlog.String("queue", m.Queue.String()),
		zlog.String("msgID", m.MsgId),
		//zlog.String("offsetMsgID", m.OffsetMsgId),
		zlog.String("msgkey", m.GetKeys()),
		zlog.String("tags", m.GetTags()),
		zlog.String("shard", m.GetShardingKey()),
		zlog.String("headers", fmtHeaders(&m.Message, HeaderPre)),
		//zlog.String("ctxHeaders", fmtHeaders(&m.Message, utils.ZYBTransportHeader)),
		zlog.Int("size", len(m.Body)),
		zlog.Int("ralCode", ralCode),
		zlog.String("response", consumeResult),
		zlog.String("retry", strconv.Itoa(int(m.ReconsumeTimes))+"/"+strconv.Itoa(c.conf.Retry)),
		zlog.Int64("delay", start.UnixNano()/int64(time.Millisecond)-m.BornTimestamp),
		zlog.String("requestStartTime", zlog.GetFormatRequestTime(start)),
		zlog.Float64("cost", zlog.GetRequestCost(start, end)),
	)

	logger.Info("rmq-consume", contextFields(ctx, fields...)...)
	if err != nil {
		logger.Error("failed to consume message: "+err.Error(), contextFields(ctx, fields...)...)
	}

	return err
}
