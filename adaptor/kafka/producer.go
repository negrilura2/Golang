package kafka

import (
	"context"
	kafkago "github.com/segmentio/kafka-go"
)

// Producer 消息生产者封装： 往指定 topic 发消息
type Producer struct {
	writer *kafkago.Writer
}

func NewProducer(brokers []string) *Producer {
	return &Producer{writer: &kafkago.Writer{
		Addr:     kafkago.TCP(brokers...),
		Balancer: &kafkago.LeastBytes{},
	}}
}

func (p *Producer) Publish(ctx context.Context, topic string, key, value []byte) error {
	return p.writer.WriteMessages(ctx, kafkago.Message{Topic: topic, Key: key, Value: value})
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
