package main

import (
	"fmt"
	"log"
	"time"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	"github.com/rabbitmq/amqp091-go"
)

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.AckType {
	return func(ps routing.PlayingState) pubsub.AckType {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
		return pubsub.Ack
	}
}

func handlerMove(gs *gamelogic.GameState, ch *amqp091.Channel) func(gamelogic.ArmyMove) pubsub.AckType {
	return func(mv gamelogic.ArmyMove) pubsub.AckType {
		defer fmt.Printf("> ")
		outcome := gs.HandleMove(mv)
		if outcome == gamelogic.MoveOutComeSafe || outcome == gamelogic.MoveOutcomeMakeWar {
			if outcome == gamelogic.MoveOutcomeMakeWar {
				war := gamelogic.RecognitionOfWar{Attacker: mv.Player, Defender: gs.GetPlayerSnap()}
				routingKey := fmt.Sprintf("%s.%s", routing.WarRecognitionsPrefix, gs.Player.Username)
				err := pubsub.PublishJSON(ch, routing.ExchangePerilTopic, routingKey, war)
				if err != nil {
					log.Println("Error publishing msg", err)
					return pubsub.NackRequeue
				}
				return pubsub.Ack
			}
			return pubsub.Ack
		}
		return pubsub.NackDiscard
	}
}

func handlerWarMsg(gs *gamelogic.GameState, ch *amqp091.Channel) func(gamelogic.RecognitionOfWar) pubsub.AckType {
	return func(war gamelogic.RecognitionOfWar) pubsub.AckType {
		defer fmt.Print("> ")
		outcome, winner, loser := gs.HandleWar(war)
		switch outcome {
		case gamelogic.WarOutcomeNotInvolved:
			return pubsub.NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			return pubsub.NackDiscard
		case gamelogic.WarOutcomeOpponentWon:
			gameLog := routing.GameLog{CurrentTime: time.Now(), Message: fmt.Sprintf("%s won  a war against %s", winner, loser), Username: gs.GetUsername()}
			err := PublishGameLog(gameLog, ch)
			if err != nil {
				log.Println("error", err.Error(), "")
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		case gamelogic.WarOutcomeYouWon:
			gamelog := routing.GameLog{CurrentTime: time.Now(), Message: fmt.Sprintf("%s won  a war against %s", winner, loser), Username: gs.GetUsername()}
			err := PublishGameLog(gamelog, ch)
			if err != nil {
				log.Println("error", err.Error())
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		case gamelogic.WarOutcomeDraw:
			gamelog := routing.GameLog{CurrentTime: time.Now(), Message: fmt.Sprintf("%s and  %s resulted in a draw", winner, loser), Username: gs.GetUsername()}
			err := PublishGameLog(gamelog, ch)
			if err != nil {
				log.Println("error", err.Error())
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		default:
			log.Println("Error in war outcome")
			return pubsub.NackDiscard
		}

	}
}
func PublishGameLog(gameLog routing.GameLog, ch *amqp091.Channel) error {
	err := pubsub.PublishGob(ch, string(routing.ExchangePerilTopic), fmt.Sprintf("%s.%s", routing.GameLogSlug, gameLog.Username), gameLog)
	return err
}
