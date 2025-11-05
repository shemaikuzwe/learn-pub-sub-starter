package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

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
	fmt.Println("Starting Peril client...")
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		log.Fatal("Failed to create conn channel")
	}
	defer ch.Close()
	username, err := gamelogic.ClientWelcome()
	if err != nil {
		log.Fatal("Hey user please provide your username next time")
	}
	queueName := fmt.Sprintf("%s.%s", routing.PauseKey, username)
	_, _, err = pubsub.DeclareAndBind(conn, routing.ExchangePerilDirect, queueName, routing.PauseKey, pubsub.TypeTransient)
	if err != nil {
		log.Fatal("Failed to create queque for a given username: ", username, err)
	}
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Good bye")
		conn.Close()
		os.Exit(1)
	}()
	gameState := gamelogic.NewGameState(username)
	err = pubsub.SubscribeJson(conn, routing.ExchangePerilDirect, queueName, routing.PauseKey, pubsub.TypeTransient, handlerPause(gameState))
	if err != nil {
		log.Fatal("Failed to subscribe to pause", err)
	}
	armyRoutingKey := "army_moves.*"
	moveKey := fmt.Sprintf("%s.%s", "army_moves", gameState.Player.Username)
	err = pubsub.SubscribeJson(conn, string(routing.ExchangePerilTopic), moveKey, armyRoutingKey, pubsub.TypeTransient, handlerMove(gameState, ch))
	if err != nil {
		log.Fatal("Failed to subscribe to move", err)
	}
	err = pubsub.SubscribeJson(conn, string(routing.ExchangePerilTopic), string(routing.WarRecognitionsPrefix), fmt.Sprintf("%s.*", routing.WarRecognitionsPrefix), pubsub.TypeDurable, handlerWarMsg(gameState, ch))
	if err != nil {
		log.Fatal("Failed to subcribe to war", err)
	}
outerLoop:
	for {
		words := gamelogic.GetInput()
		if len(words) == 0 {
			continue
		}
		command := words[0]
		switch command {
		case "spawn":
			err := gameState.CommandSpawn(words)
			if err != nil {
				log.Println("We got an error try again", err)
				continue
			}
		case "move":
			move, err := gameState.CommandMove(words)
			if err != nil {
				log.Println("We got an error try again", err)
				continue
			}
			err = pubsub.PublishJSON(ch, routing.ExchangePerilTopic, moveKey, move)
			if err != nil {
				log.Println("failed to publish json")
			}
			log.Printf("move %s %d", move.ToLocation, len(move.Units))
		case "status":
			gameState.CommandStatus()

		case "help":
			gamelogic.PrintClientHelp()
		case "spam":
			spam, err := strconv.Atoi(words[1])
			if err != nil {
				log.Println("Failed to conver to an int")
				continue
			}
			for range spam {
				malicousLog := gamelogic.GetMaliciousLog()
				gameLog := routing.GameLog{CurrentTime: time.Now(), Message: malicousLog, Username: gameState.GetUsername()}
				err := PublishGameLog(gameLog, ch)
				if err != nil {
					log.Println("error", err)
					continue
				}

			}
		case "quit":
			{
				gamelogic.PrintQuit()
				break outerLoop
			}
		default:
			log.Println("You entered wrong command,use help command to see available commands")
		}
	}

}
