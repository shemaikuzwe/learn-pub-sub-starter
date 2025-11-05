package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	connString := "amqp://guest:guest@localhost:5672/"
	conn, err := amqp.Dial(connString)
	if err != nil {
		log.Fatal("Failed to connect to ampq server")
	}
	defer conn.Close()
	log.Println("Connected to ampq server ")
	fmt.Println("Starting Peril server...")
	ch, err := conn.Channel()
	if err != nil {
		log.Fatal("Failed to create channel")
	}
	_, _, err = pubsub.DeclareAndBind(conn, routing.ExchangePerilTopic, routing.GameLogSlug, routing.GameLogSlug+".*", pubsub.TypeDurable)
	if err != nil {
		log.Fatal("Failed to create queue", err)
	}
	defer ch.Close()

	key := fmt.Sprintf("%s.*", routing.GameLogSlug)
	err = pubsub.SubscribeGob(conn, routing.ExchangePerilTopic, routing.GameLogSlug, key, pubsub.TypeDurable, GameLogHandler())
	if err != nil {
		log.Fatal("Failed to subscribe to game log queue")
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutting down")
		conn.Close()
		os.Exit(1)
	}()
	gamelogic.PrintServerHelp()
outerLoop:
	for {
		input := gamelogic.GetInput()
		if len(input) == 0 {
			continue
		}
		command := input[0]
		switch command {
		case "pause":
			log.Println("Pausing the game")
			err = pubsub.PublishJSON(ch, string(routing.ExchangePerilDirect), string(routing.PauseKey), routing.PlayingState{IsPaused: true})
			if err != nil {
				log.Println("err", err)
			}
		case "resume":
			log.Println("Resuming the game")
			err = pubsub.PublishJSON(ch, string(routing.ExchangePerilDirect), string(routing.PauseKey), routing.PlayingState{IsPaused: false})
			if err != nil {
				log.Println("err", err)
			}
		case "exit":
			log.Println("Exiting the game")
			break outerLoop
		default:
			log.Println("Invalid command")
		}
	}

}
