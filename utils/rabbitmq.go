package utils

import (
	"context"
	"encoding/json"
	"time"

	"github.com/streadway/amqp"
)

var RabbitMQConn *amqp.Connection
var RabbitMQChannel *amqp.Channel

// InitRabbitMQ 初始化RabbitMQ
func InitRabbitMQ(url string) error {
	// 建立连接
	conn, err := amqp.Dial(url)
	if err != nil {
		return err
	}
	RabbitMQConn = conn

	// 建立通道
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	RabbitMQChannel = ch

	// 声明交换机和队列
	err = declareExchangeAndQueue()
	if err != nil {
		return err
	}

	return nil
}

// 声明交换机和队列（交易执行队列）
func declareExchangeAndQueue() error {
	// 声明交换机
	err := RabbitMQChannel.ExchangeDeclare(
		"nft_trade_exchange", // 交换机名
		"direct",             // 类型
		true,                 // 持久化
		false,                // 自动删除
		false,                // 内部
		false,                // 等待
		nil,                  // 参数
	)
	if err != nil {
		return err
	}

	// 声明队列
	_, err = RabbitMQChannel.QueueDeclare(
		"nft_trade_queue", // 队列名
		true,              // 持久化
		false,             // 自动删除
		false,             // 排他
		false,             // 等待
		nil,               // 参数
	)
	if err != nil {
		return err
	}

	// 绑定队列到交换机
	err = RabbitMQChannel.QueueBind(
		"nft_trade_queue",    // 队列名
		"trade.execute",      // 路由键
		"nft_trade_exchange", // 交换机名
		false,
		nil,
	)
	if err != nil {
		return err
	}

	return nil
}

// PublishTradeMsg 发布交易执行消息
func PublishTradeMsg(ctx context.Context, orderNo string) error {
	// 序列化消息
	msg, err := json.Marshal(map[string]string{"order_no": orderNo})
	if err != nil {
		return err
	}

	// 发布消息
	err = RabbitMQChannel.Publish(
		"nft_trade_exchange", // 交换机名
		"trade.execute",      // 路由键
		false,                // 强制
		false,                // 立即
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         msg,
			DeliveryMode: amqp.Persistent, // 持久化
			Timestamp:    time.Now(),
		},
	)
	return err
}

// ConsumeTradeMsg 消费交易执行消息
func ConsumeTradeMsg(handler func(orderNo string) error) error {
	msgs, err := RabbitMQChannel.Consume(
		"nft_trade_queue", // 队列名
		"",                // 消费者标签
		false,             // 自动确认
		false,             // 排他
		false,             // 不本地
		false,             // 等待
		nil,               // 参数
	)
	if err != nil {
		return err
	}

	// 启动协程消费消息
	go func() {
		for d := range msgs {
			// 反序列化消息
			var msg map[string]string
			err := json.Unmarshal(d.Body, &msg)
			if err != nil {
				Logger.Error("消息反序列化失败", zap.Error(err))
				d.Nack(false, false) // 拒绝消息，不重新入队
				continue
			}

			orderNo, ok := msg["order_no"]
			if !ok {
				Logger.Error("消息缺少order_no")
				d.Nack(false, false)
				continue
			}

			// 处理消息
			err = handler(orderNo)
			if err != nil {
				Logger.Error("处理交易消息失败", zap.String("order_no", orderNo), zap.Error(err))
				d.Nack(false, true) // 拒绝消息，重新入队
			} else {
				d.Ack(false) // 确认消息
			}
		}
	}()

	return nil
}

// CloseRabbitMQ 关闭RabbitMQ连接
func CloseRabbitMQ() {
	if RabbitMQChannel != nil {
		RabbitMQChannel.Close()
	}
	if RabbitMQConn != nil {
		RabbitMQConn.Close()
	}
}
