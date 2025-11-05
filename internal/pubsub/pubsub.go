package pubsub

import (
	"context"
	"encoding/json"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	data, err := json.Marshal(val)
	if err != nil {
		return err
	}
	err = ch.PublishWithContext(context.Background(), exchange, key, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        data,
	})
	return err
}

type SimpleQueueType string
type AckType string

const (
	TypeDurable   SimpleQueueType = "durable"
	TypeTransient SimpleQueueType = "transient"
)

const (
	Ack         AckType = "Ack"
	NackRequeue AckType = "NackRequeue"
	NackDiscard AckType = "NackDiscard"
)

func DeclareAndBind(
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType SimpleQueueType, // an enum to represent "durable" or "transient"
) (*amqp.Channel, amqp.Queue, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, amqp.Queue{}, err
	}
	queue, err := ch.QueueDeclare(
		queueName,
		queueType == TypeDurable,
		queueType == TypeTransient,
		queueType == TypeTransient,
		false,
		amqp.Table{"x-dead-letter-exchange": "peril_dlx"},
	)
	if err != nil {
		return nil, amqp.Queue{}, err
	}
	err = ch.QueueBind(queue.Name, key, exchange, false, nil)
	if err != nil {
		return nil, amqp.Queue{}, err
	}
	return ch, queue, nil
}

func SubscribeJson[T any](conn *amqp.Connection, exchange, queueName, key string, queueType SimpleQueueType, handler func(T) AckType) error {
	_, _, err := DeclareAndBind(conn, exchange, queueName, key, queueType)
	if err != nil {
		return err
	}
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	err = ch.Qos(10, 0, false)
	if err != nil {
		return err
	}
	delivery, err := ch.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		return err
	}
	go func() {
		for message := range delivery {
			var msg T
			err := json.Unmarshal(message.Body, &msg)
			if err != nil {
				log.Println("Failed to parse json", err)
				continue
			}
			ack := handler(msg)
			switch ack {
			case Ack:
				message.Ack(false)
			case NackRequeue:
				message.Nack(false, true)
			default:
				message.Nack(false, false)
			}
		}
	}()

	return nil
}
