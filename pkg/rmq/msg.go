package rmq

import (
	"bytes"
	"github.com/tiant-go/golib/pkg/zlog"
	"strings"
	"time"

	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/gin-gonic/gin"
)

// DelayLevel 定义消息延迟发送的级别
type DelayLevel int

const (
	Second = DelayLevel(iota + 1)
	Seconds5
	Seconds10
	Seconds30
	Minute1
	Minutes2
	Minutes3
	Minutes4
	Minutes5
	Minutes6
	Minutes7
	Minutes8
	Minutes9
	Minutes10
	Minutes20
	Minutes30
	Hour1
	Hours2
)

const HeaderPre = "X-Mq-"

// NewMessage 创建一条新的消息
func NewMessage(service string, content []byte) (Message, error) {
	if svc, exist := producerService.Load(service); exist {
		producer := svc.(*Producer)
		return &messageWrapper{
			producer: producer,
			msg:      primitive.NewMessage(producer.conf.Topic, content),
		}, nil
	}
	return nil, ErrRmqSvcNotRegiestered
}

// Message 消息提供的接口定义
type Message interface {
	WithTag(string) Message
	WithKey(string) Message
	WithShard(string) Message
	WithDelay(DelayLevel) Message
	WithHeader(key string, value string) Message
	Send(ctx *gin.Context) (msgID string, err error)
	GetContent() []byte
	GetTag() string
	GetKey() string
	GetShard() string
	GetID() string
	GetHeader(key string) string
	GetAllHeader() map[string]string
	GetTime() time.Time //消费时使用  获取消息生产时间
	GetRetry() int      //消费时使用  获取消息重试次数
	GetTopic() string   //消费时使用  获取消息Topic

	// Deprecated: nmq hack方法, 不需使用
	SetTopic(string) Message
	// Deprecated: nmq hack方法, 不需使用
	SetProperty(key string, value string) Message //nmq hack方法, 不需使用
}

type messageWrapper struct {
	msg      *primitive.Message
	producer *Producer
	msgID    string
	time     time.Time //消息生产时间
	retry    int       //消息重试次数
}

// WithTag 设置消息的标签Tag
func (m *messageWrapper) WithTag(tag string) Message {
	m.msg = m.msg.WithTag(tag)
	return m
}

// WithKey 设置消息的业务ID
func (m *messageWrapper) WithKey(key string) Message {
	m.msg = m.msg.WithKeys([]string{key})
	return m
}

// WithShard 设置消息的分片键
func (m *messageWrapper) WithShard(shard string) Message {
	m.msg = m.msg.WithShardingKey(shard)
	return m
}

// WithHeader 设置自定义消息头
// key: 使用 中划线+各段首字母大写 的标准形式 eg: Key/Long-Key，否则 golib 内部会转为该形式
//
//	获取 header 时请使用标准形式获取
func (m *messageWrapper) WithHeader(key string, value string) Message {
	m.msg.WithProperty(key2Header(key), value)
	return m
}

// WithDelay 设置消息的延迟等级
func (m *messageWrapper) WithDelay(lvl DelayLevel) Message {
	m.msg = m.msg.WithDelayTimeLevel(int(lvl))
	return m
}

// Send 发送消息
func (m *messageWrapper) Send(ctx *gin.Context) (msgID string, err error) {
	fields := []zlog.Field{
		zlog.String("method", "produce"),
		zlog.String("service", m.producer.conf.Service),
		zlog.String("addr", m.producer.conf.NameServer),
		zlog.String("topic", m.msg.Topic),
	}

	if m.producer == nil {
		logger.Error("client is not specified", contextFields(ctx, fields...)...)
		return "", ErrRmqSvcInvalidOperation
	}

	start := time.Now()
	prod := m.producer.producer
	if prod == nil {
		logger.Error("producer not started", contextFields(ctx, fields...)...)
		return "", ErrRmqSvcInvalidOperation
	}

	queue, msgID, _, err := m.producer.sendMessage(m.msg)

	ralCode := 0
	msg := "sent message success"
	if err != nil {
		ralCode = -1
		msg = err.Error()
	}

	end := time.Now()
	var bodyLog bytes.Buffer
	if len(m.msg.Body) > 100 {
		bodyLog.Write(m.msg.Body[0:100])
		bodyLog.WriteString("...")
	} else {
		bodyLog.Write(m.msg.Body)
	}

	fields = append(fields, []zlog.Field{
		zlog.String("message", bodyLog.String()),
		zlog.String("queue", queue),
		zlog.String("msgID", msgID),
		zlog.String("msgkey", m.msg.GetKeys()),
		zlog.String("tags", m.msg.GetTags()),
		zlog.String("shard", m.msg.GetShardingKey()),
		zlog.String("headers", fmtHeaders(m.msg, HeaderPre)),
		zlog.Int("size", len(m.msg.Body)),
		zlog.Int("ralCode", ralCode),
		zlog.String("response", msg),
		zlog.String("requestStartTime", zlog.GetFormatRequestTime(start)),
		zlog.Float64("cost", zlog.GetRequestCost(start, end)),
	}...)
	logger.Info("rmq-produce", contextFields(ctx, fields...)...)
	if err != nil {
		logger.Error("failed to send message: "+err.Error(), contextFields(ctx, fields...)...)
	}

	return msgID, err
}

// GetContent 获取消息体内容
func (m *messageWrapper) GetContent() []byte {
	return m.msg.Body
}

// GetTag 获取消息标签
func (m *messageWrapper) GetTag() string {
	return m.msg.GetTags()
}

// GetKey 获取消息业务ID
func (m *messageWrapper) GetKey() string {
	return m.msg.GetKeys()
}

// GetHeader 获取自定义消息头
// key: 应该为 中划线+各段首字母大写 的标准形式 eg: Key/Long-Key
func (m *messageWrapper) GetHeader(key string) string {
	return m.msg.GetProperty(key2Header(key))
}

// SetTopic 设置消息Topic, hack方法, 支持nmq边车的生产功能
func (m *messageWrapper) SetTopic(topic string) Message {
	m.msg.Topic = topic
	return m
}

// SetProperty 设置消息Topic, hack方法, 支持nmq边车的生产功能
func (m *messageWrapper) SetProperty(key string, value string) Message {
	m.msg.WithProperty(key, value)
	return m
}

// GetAllHeader 获取全部自定义消息头
// 返回 map 的 key 为 中划线+各段首字母大写 的标准形式 eg: Key/Long-Key
func (m *messageWrapper) GetAllHeader() map[string]string {
	headers := make(map[string]string)
	for key, value := range m.msg.GetProperties() {
		if strings.HasPrefix(key, HeaderPre) {
			headers[header2Key(key)] = value
		}
	}
	return headers
}

// GetShard 获取消息分片键
func (m *messageWrapper) GetShard() string {
	return m.msg.GetShardingKey()
}

// GetID 获取消息ID
func (m *messageWrapper) GetID() string {
	return m.msgID
}

// GetTime 获取消息生产时间
func (m *messageWrapper) GetTime() time.Time {
	return m.time
}

// GetRetry 获取消息重试次数
func (m *messageWrapper) GetRetry() int {
	return m.retry
}

// GetTopic 获取消息主题
func (m *messageWrapper) GetTopic() string {
	return m.msg.Topic
}

type MessageBatch []Message

func (batch MessageBatch) Send(ctx *gin.Context) (msgID string, err error) {
	if len(batch) == 0 {
		logger.Error("message batch is empty", contextFields(ctx)...)
		return "", ErrRmqSvcInvalidOperation
	}

	firstMsg := batch[0].(*messageWrapper)
	producer := firstMsg.producer

	var msgs = make([]*primitive.Message, 0)
	for _, m := range batch {
		// 校验 topic 是同一个
		mw := m.(*messageWrapper)
		if mw.producer != producer {
			logger.Error("message batch must point to same producer", contextFields(ctx)...)
			return "", ErrRmqSvcInvalidOperation
		}
		msgs = append(msgs, mw.msg)
	}

	start := time.Now()
	queue, _, offset, err := producer.sendMessage(msgs...)

	ralCode := 0
	msg := "sent message batch success"
	if err != nil {
		ralCode = -1
		msg = "failed to send message batch : " + err.Error()
		logger.Error("failed to send message batch", contextFields(ctx, zlog.String("error", err.Error()))...)
	}

	end := time.Now()

	fields := []zlog.Field{
		zlog.String("method", "produce"),
		zlog.String("service", producer.conf.Service),
		zlog.String("addr", producer.conf.NameServer),
		zlog.String("topic", firstMsg.msg.Topic),
		zlog.String("queue", queue),
		zlog.String("msgID", offset),
		zlog.Int("batch", len(msgs)),
		zlog.Int("ralCode", ralCode),
		zlog.String("response", msg),
		zlog.String("requestStartTime", zlog.GetFormatRequestTime(start)),
		zlog.Float64("cost", zlog.GetRequestCost(start, end)),
	}

	logger.Info("rmq-produce", contextFields(ctx, fields...)...)

	// batch send will not set msgid
	// offsetid is the join of all msg's offsetid
	return offset, err
}
