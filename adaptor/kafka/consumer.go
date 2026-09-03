package kafka

import (
	"context"
	kafkago "github.com/segmentio/kafka-go"
	"go.uber.org/zap"
	"mall/utils/logger"
	"time"
)

func RunConsumer(ctx context.Context, brokers []string, topic, groupID string, handler func(ctx context.Context, value []byte) error) error {
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
		err = handler(ctx, msg.Value) //handler 一直失败就会无限重试同一条消息，把分区后面所有的消息都堵住--生产中会给“重试次数上限 + 死信队列（DLQ）“。这个项目中先靠幂等+无限重试兜底。
		if err != nil {
			logger.Error("RunConsumer handler error", zap.Error(err))
			time.Sleep(2 * time.Second)
			continue
		}
		err = reader.CommitMessages(ctx, msg)
		if err != nil {
			logger.Error("RunConsumer CommitMessages error", zap.Error(err))
			time.Sleep(2 * time.Second)
			continue
		}

	}
}
