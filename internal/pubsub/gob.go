package pubsub

import (
	"bytes"
	"context"
	"encoding/gob"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

func PublishGob[T any](ch *amqp.Channel, exchange, key string, val T) error {
	data, err := encode(val)
	if err != nil {
		return err
	}
	err = ch.PublishWithContext(context.Background(), exchange, key, false, false, amqp.Publishing{
		ContentType: "application/gob",
		Body:        data,
	})
	return err
}

func encode[T any](val T) ([]byte, error) {
	var buff bytes.Buffer
	encoder := gob.NewEncoder(&buff)
	err := encoder.Encode(val)
	if err != nil {
		return nil, err
	}
	return buff.Bytes(), nil
}
func decode[T any](data []byte, s *T) error {
	reader := bytes.NewReader(data)
	decoder := gob.NewDecoder(reader)
	err := decoder.Decode(&s)
	return err
}
func SubscribeGob[T any](conn *amqp.Connection, exchange, queueName, key string, queueType SimpleQueueType, handler func(T) AckType) error {
	_, _, err := DeclareAndBind(conn, exchange, queueName, key, queueType)
	if err != nil {
		return err
	}
	ch, err := conn.Channel()
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
			err := decode(message.Body, &msg)
			if err != nil {
				log.Println("Failed to parse json", err)
				continue
			}
			ack := handler(msg)
			log.Println("ack", ack)
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
