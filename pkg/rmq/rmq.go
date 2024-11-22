// Package rmq 提供了访问Rmq服务的能力
package rmq

import (
	"fmt"
	"github.com/xiangtao94/golib/pkg/env"
	"github.com/xiangtao94/golib/pkg/rmq/auth"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

const (
	batchMax       = 1024
	instancePrefix = "rmq-"

	defaultTraceTopic = "RMQ_SYS_TRACE_TOPIC"
)

var (
	// ErrRmqSvcConfigInvalid 服务配置无效
	ErrRmqSvcConfigInvalid = fmt.Errorf("requested rmq service is not correctly configured")
	// ErrRmqSvcNotRegiestered 服务尚未被注册
	ErrRmqSvcNotRegiestered = fmt.Errorf("requested rmq service is not registered")
	// ErrRmqSvcAlreadyRegistered 服务已被注册
	ErrRmqSvcAlreadyRegistered = fmt.Errorf("requested rmq service is already registered")
	// ErrRmqSvcInvalidOperation 当前操作无效
	ErrRmqSvcInvalidOperation = fmt.Errorf("requested rmq service is not suitable for current operation")
)

var (
	producerService = &sync.Map{} //map[string]*client
	consumeService  = &sync.Map{}
)

// MessageCallback 定义业务方接收消息的回调接口
type MessageCallback func(ctx *gin.Context, msg Message) error

// MessageBatchCallback 暂未实现，定义业务方接收消息的回调接口(批量处理消息形式)，
// type MessageBatchCallback func(ctx *gin.Context, msg ...Message) error

// auth 提供链接到Broker所需要的验证信息（按需配置）
type Auth struct {
	AccessKey string `json:"ak,omitempty" yaml:"ak,omitempty"`
	SecretKey string `json:"sk,omitempty" yaml:"sk,omitempty"`
}

type ClientConf struct {
	// 名称，不同 producer 间不可重复，不同 consumer 间不可重复
	Service string `json:"service" yaml:"service"`
	// 提供名字服务器的地址，eg: mq-xxx-svc.mq
	NameServer string `json:"nameserver" yaml:"nameserver"`
	// auth 配置，走 proxy 鉴权，通常不需要手动配置
	Auth Auth `json:"auth" yaml:"auth"`
	// 要生产/消费的主题
	Topic string `json:"topic" yaml:"topic"`
	// 是否开启消息轨迹，默认不开启
	Trace bool `json:"trace" yaml:"trace"`
	// 存储消息轨迹的 topic, 默认： RMQ_SYS_TRACE_TOPIC
	TraceTopic string `json:"traceTopic" yaml:"traceTopic"`
}

type ProducerConf struct {
	ClientConf `json:",inline" yaml:",inline"`
	// 生产重试次数
	Retry int `json:"retry" yaml:"retry"`
	// 生产超时时间
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

type ConsumerConf struct {
	ClientConf `json:",inline" yaml:",inline"`
	// 消费消息的TAG
	Tags []string `json:"tags" yaml:"tags"`
	// 消费组名称
	Group string `json:"group" yaml:"group"`
	// 是否是广播消费模式
	Broadcast bool `json:"broadcast" yaml:"broadcast"`
	// 是否是顺序消费模式
	Orderly bool `json:"orderly" yaml:"orderly"`
	// 批量消费数量, 默认1
	Batch int `json:"batch" yaml:"batch"`
	// 消费失败时的重试次数
	Retry int `json:"retry" yaml:"retry"`
	// 消费失败重试间隔  顺序消费时可用
	RetryInterval time.Duration `json:"retry_interval,omitempty" yaml:"retry_interval,omitempty"`
}

type RmqConfig struct {
	Producer []ProducerConf
	Consumer []ConsumerConf
}

func (conf *ClientConf) Check() error {

	if conf.Service == "" {
		return errors.Wrap(ErrRmqSvcConfigInvalid, "service not specified")
	}
	if conf.Topic == "" {
		return errors.Wrap(ErrRmqSvcConfigInvalid, "topic not specified")
	}
	if conf.NameServer == "" {
		return errors.Wrap(ErrRmqSvcConfigInvalid, "nameserver not specified")
	}

	if conf.TraceTopic == "" {
		conf.TraceTopic = defaultTraceTopic
	}

	return nil
}

func (conf *ConsumerConf) Check() error {
	err := conf.ClientConf.Check()
	if err != nil {
		return err
	}

	if conf.Group == "" {
		return errors.Wrap(ErrRmqSvcConfigInvalid, "consumer group not specified")
	}
	// 不能同时配置顺序消费和广播消费
	if conf.Broadcast && conf.Orderly {
		return errors.Wrap(ErrRmqSvcConfigInvalid, "can not use orderly with broadcast")
	}
	if conf.Batch <= 0 { //未配置, 则默认为1
		conf.Batch = 1
	}
	if conf.Batch > batchMax {
		return errors.Wrapf(ErrRmqSvcConfigInvalid, "batch size greater than max batch limit:%d", batchMax)
	}
	if conf.RetryInterval < time.Millisecond*10 {
		conf.RetryInterval = time.Millisecond * 10 //重试间隔最低10ms
	}
	if conf.Auth.AccessKey == "" {
		id := auth.NewIdentity(env.GetAppName(), conf.Group, auth.ClientTypeConsumer)
		conf.Auth.AccessKey, conf.Auth.SecretKey = id.Credential()
	}

	return nil
}

func (conf *ProducerConf) Check() error {
	err := conf.ClientConf.Check()
	if err != nil {
		return err
	}

	if conf.Timeout == 0 {
		conf.Timeout = time.Millisecond * 1000 //默认1000ms
	}
	// 优先级：配置 > 环境变量
	// 未配置 ak 且添加了相关环境变量则会自动生成 aksk
	if conf.Auth.AccessKey == "" {
		id := auth.NewIdentity(env.GetAppName(), "DEFAULT_PRODUCER", auth.ClientTypeProducer)
		conf.Auth.AccessKey, conf.Auth.SecretKey = id.Credential()
	}

	return nil
}

func InitProducer(config ProducerConf) (err error) {
	// 不允许重复注册 service 字段相同的 producer
	if _, exist := producerService.Load(config.Service); exist {
		return ErrRmqSvcAlreadyRegistered
	}

	if err := config.Check(); err != nil {
		return err
	}

	initLogger()

	producer, err := newProducer(config)
	if err != nil {
		return err
	}
	producerService.Store(config.Service, producer)

	return nil
}

// StartProducer 启动指定已注册的Rmq生产服务
func StartProducer(service string) error {
	producer, ok := producerService.Load(service)
	if !ok {
		return ErrRmqSvcNotRegiestered
	}

	return producer.(*Producer).start()
}

// StopProducer 停止指定已注册的Rmq生产服务
func StopProducer(service string) error {
	producer, ok := producerService.Load(service)
	if !ok {
		return ErrRmqSvcNotRegiestered
	}

	if producer.(*Producer).producer == nil {
		return ErrRmqSvcInvalidOperation
	}

	err := producer.(*Producer).stop()
	producer.(*Producer).producer = nil
	producerService.Delete(service)
	return err
}

func InitConsumer(config ConsumerConf) (err error) {
	// 不允许重复注册或注册 service 字段相同的 consumer
	if _, exist := consumeService.Load(config.Service); exist {
		return ErrRmqSvcAlreadyRegistered
	}

	if err = config.Check(); err != nil {
		return err
	}

	initLogger()

	consumer, err := newPushConsumer(config)
	if err != nil {
		return err
	}
	consumeService.Store(config.Service, consumer)
	return nil
}

// StartConsumer 启动指定已注册的Rmq消费服务， 并指定消费回调
func StartConsumer(g *gin.Engine, service string, callback MessageCallback) error {
	svc, exist := consumeService.Load(service)
	if !exist {
		return ErrRmqSvcNotRegiestered
	}
	consumer := svc.(*pushConsumer)

	if consumer.engine == nil {
		consumer.engine = g
	}

	// 初始化 PushConsumer
	return consumer.start(callback)
}

// StopConsumer 停止指定已注册的Rmq消费服务
func StopConsumer(service string) error {
	svc, exist := consumeService.Load(service)
	if !exist {
		return ErrRmqSvcNotRegiestered
	}
	consumer := svc.(*pushConsumer)

	if consumer == nil {
		return ErrRmqSvcInvalidOperation
	}

	err := consumer.stop()
	consumer = nil
	consumeService.Delete(service)
	return err
}

// 停止所有 rmq 生产者
func StopRmqProduce() {
	producerService.Range(func(service, producer interface{}) bool {
		_ = StopProducer(service.(string))
		return true
	})
}

// 停止所有 rmq 消费者
func StopRmqConsume() {
	consumeService.Range(func(service, consumer interface{}) bool {
		_ = StopConsumer(service.(string))
		return true
	})
}
