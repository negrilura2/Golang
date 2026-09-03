package timer

import (
	"github.com/go-co-op/gocron"
	"mall/adaptor"
	"mall/adaptor/kafka"
	"mall/adaptor/redis"
	"mall/adaptor/repo/outbox"
	"mall/service/order"

	"mall/config"
	"time"
)

type Service struct {
	conf     *config.Config
	schedule *gocron.Scheduler
	order    *order.Service
	rdsOrder redis.IOrder
	outbox   outbox.IOutbox
	producer *kafka.Producer
}

func NewService(adaptor adaptor.IAdaptor) *Service {
	return &Service{
		conf:     adaptor.GetConfig(),
		schedule: gocron.NewScheduler(time.Local),
		order:    order.NewService(adaptor),
		rdsOrder: redis.NewOrder(adaptor),
		outbox:   outbox.NewOutbox(adaptor),
		producer: kafka.NewProducer(adaptor.GetConfig().Kafka.Brokers),
	}
}

func (s *Service) Start() {
	// 待支付订单的超时取消动作，每分钟检查一次
	_, err := s.schedule.Every(1).Minute().Do(s.OrderTimeOutCancel)
	if err != nil {
		panic(err)
	}
	// 优先等待支付回调，订单支付结果主动轮询，每5s中我就查询一次，定时和回调谁先到，谁处理订单支付结果
	_, err = s.schedule.Every(5).Seconds().Do(s.OrderPayResultQuery)
	if err != nil {
		panic(err)
	}

	// 订单退款查询
	_, err = s.schedule.Every(5).Seconds().Do(s.OrderRefundQuery)
	if err != nil {
		panic(err)
	}

	//搬运工： 每5s把 outbox 待发布事件搬进 kafka topic
	_, err = s.schedule.Every(5).Seconds().Do(s.OrderEventPublish)
	if err != nil {
		panic(err)
	}
	
	s.schedule.StartAsync() // 异步启动，不阻塞
}

func (s *Service) Stop() {
	s.schedule.Stop()
	if s.producer != nil {
		_ = s.producer.Close() //进程优雅退出时把连接还掉
	}
}
