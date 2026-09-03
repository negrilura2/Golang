package kafka

import (
	"context"
	kafkago "github.com/segmentio/kafka-go"
	"go.uber.org/zap"
	"mall/consts"
	"mall/utils/logger"
	"time"
)

func RunConsumer(ctx context.Context, brokers []string, topic, groupID string, handler func(ctx context.Context, value []byte) error, dlqTopic string) error {
	var dlqProducer *Producer
	if dlqTopic != "" {
		dlqProducer = NewProducer(brokers)
		defer dlqProducer.Close()
	}
	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:     brokers,
		GroupID:     groupID,
		Topic:       topic,
		StartOffset: kafkago.FirstOffset,
	})
	defer reader.Close()

	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				logger.Error("RunConsumer FetchMessage error", zap.Error(err))
				time.Sleep(2 * time.Second)
				continue //瞬时错误自愈，不自杀
			}
		}
		//重试同一条，有上限
		err = handler(ctx, msg.Value)
		for attempt := 1; attempt <= consts.MaxConsumeRetry && err != nil; attempt++ {
			logger.Error("RunConsumer handler error (retry)", zap.Int("attempt", attempt), zap.Error(err))
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Duration(attempt) * time.Second): //退避: 1s, 2s, 3s
			}
			err = handler(ctx, msg.Value) //还是同一条
		}
		//重试耗尽： 毒消息/持续故障
		if err != nil {
			if dlqProducer != nil {
				//生产：投死信队列mall.order.payed.dlq + 报警， 再提交， 别堵
				if perr := dlqProducer.Publish(ctx, dlqTopic, msg.Key, msg.Value); perr != nil {
					// 投死信也失败，没兜住。 提交=真丢 - 不提交， 让consumer 停下来报错， 重启后这条会直接重投
					logger.Error("RunConsumer publish to DLQ error", zap.String("dlq_topic", dlqTopic), zap.Error(perr))
					return perr
				} else {
					logger.Error("RunConsumer give up after retries, message send to DLQ", zap.String("dlq_topic", dlqTopic), zap.Error(err))
				}
			} else {
				logger.Error("RunConsumer give up after retries (no DLQ), message dropped", zap.Error(err))
			}
		}
		if err = reader.CommitMessages(ctx, msg); err != nil {
			logger.Error("RunConsumer CommitMessages error", zap.Error(err))
			time.Sleep(2 * time.Second)
			continue
		}

	}
}
