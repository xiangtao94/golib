package rmq

import (
	"context"
	"github.com/xiangtao94/golib/pkg/env"
	"github.com/xiangtao94/golib/pkg/zlog"
	"hash/fnv"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"
	"github.com/pkg/errors"
)

type Producer struct {
	conf     ProducerConf
	producer rocketmq.Producer
}

func newProducer(conf ProducerConf) (*Producer, error) {
	rander := rand.New(rand.NewSource(time.Now().UTC().UnixNano()))
	nameServerAddr := getHostListByDns([]string{conf.NameServer})

	instance := instancePrefix + conf.Service
	options := []producer.Option{
		producer.WithInstanceName(instance + "-" + env.LocalIP + "-" + strconv.Itoa(os.Getpid()) + "-producer"),
		producer.WithNsResolver(primitive.NewPassthroughResolver(nameServerAddr)),
		producer.WithRetry(conf.Retry),
		producer.WithQueueSelector(&queueSelectorByShardingKey{Rander: rander}),
	}
	// 消息轨迹
	if conf.Trace {
		options = append(options, producer.WithTrace(&primitive.TraceConfig{
			TraceTopic: conf.TraceTopic,
			Access:     primitive.Local,
			Resolver:   primitive.NewPassthroughResolver(nameServerAddr),
		}))
	}

	if conf.Auth.AccessKey != "" && conf.Auth.SecretKey != "" {
		options = append(options, producer.WithCredentials(primitive.Credentials{
			AccessKey: conf.Auth.AccessKey,
			SecretKey: conf.Auth.SecretKey,
		}))
	}
	if conf.Timeout != 0 {
		options = append(options, producer.WithSendMsgTimeout(conf.Timeout))
	}
	prod, err := rocketmq.NewProducer(options...)
	if err != nil {
		logger.Error("failed to create producer",
			fields(zlog.String("ns", conf.NameServer), zlog.String("error", err.Error()))...)
		return nil, err
	}

	return &Producer{
		conf:     conf,
		producer: prod,
	}, nil
}

func (p *Producer) start() error {
	if p.producer == nil {
		return errors.Wrap(ErrRmqSvcInvalidOperation, "producer not initialized")
	}

	err := p.producer.Start()
	if err != nil {
		logger.Error("failed to start consumer", fields(zlog.String("error", err.Error()))...)
		return err
	}

	return nil
}

func (p *Producer) stop() error {
	return p.producer.Shutdown()
}

func (p *Producer) sendMessage(msgs ...*primitive.Message) (string, string, string, error) {
	res, err := p.producer.SendSync(context.Background(), msgs...)
	if err != nil {
		logger.Error("failed to send messages", fields(zlog.String("error", err.Error()))...)
		return "", "", "", err
	}
	return res.MessageQueue.String(), res.MsgID, res.OffsetMsgID, err
}

// ShardingKey hash方法
type queueSelectorByShardingKey struct {
	Rander *rand.Rand
}

func (q *queueSelectorByShardingKey) Select(msg *primitive.Message, queues []*primitive.MessageQueue, lastBrokerName string) *primitive.MessageQueue {
	if msg.GetShardingKey() == "" {
		return queues[q.Rander.Intn(len(queues))]
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(msg.GetShardingKey()))
	return queues[h.Sum32()%uint32(len(queues))]
}

type NmqResponse struct {
	TransID uint64 `mcpack:"_transid"`
	ErrNo   int    `mcpack:"_error_no" binding:"required"`
	ErrStr  string `mcpack:"_error_msg" binding:"required"`
}
