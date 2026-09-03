package main

import (
	"context"
	"fmt"
	"github.com/segmentio/kafka-go"
	"time"
)

func main() {
	//topic = 频道名（消息的栏目）。 Kafka 默认自动建， 我们不用手动建。
	topic := "mall.order.payed"
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	//=======================1) 生产者： 往频道里发一条消息 =================
	writer := &kafka.Writer{
		Addr:     kafka.TCP("localhost:9092"), //连接docker Kafka
		Topic:    topic,                       //发到哪个频道
		Balancer: &kafka.LeastBytes{},         //多分区时消息怎么均衡（先不管）
	}
	defer writer.Close()
	err := writer.WriteMessages(ctx, kafka.Message{
		Value: []byte("hello, 支付成功 #1"), //消息内容（业务这里就是事件JSON）
	})
	if err != nil {
		fmt.Println("x 生产者发送失败:", err)
		return
	}
	fmt.Println("√ 生产者已发送")

	// ==================2)消费者：从频道里收消息 =================
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{"localhost:9092"},
		GroupID:     "demo-group", //消费组： 同组内一条消息只能被消费一次
		Topic:       topic,
		StartOffset: kafka.FirstOffset, //新消费组从最早的消息开始读
	})
	defer reader.Close()

	msg, err := reader.ReadMessage(ctx) //阻塞等待下一条消息
	if err != nil {
		fmt.Println("X 消费者读取失败：", err)
		return
	}
	fmt.Printf("√ 消费者收到：topic=%s partition=%d offset=%d value=%s\n", msg.Topic, msg.Partition, msg.Offset, string(msg.Value))
}
