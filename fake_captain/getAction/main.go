package main

import (
	"errors"
	"log"

	rabbitmqClient "github.com/ray31245/AGH3-Action/pkg/rabbitmq_client"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}

func main() {
	client, err := rabbitmqClient.New("amqp://guest:guest@localhost:5672/")
	failOnError(err, "Failed to create rabbitmqClient")

	res, err := client.RPC().GetAction(rabbitmqClient.GetActionRequest{
		Selector: rabbitmqClient.SelectOne{Name: "foo"},
	})

	log.Printf(" [.] Got Error %v", err)
	log.Println(res)
	log.Println(res.Action.Name)
	log.Println(res.Status)
	log.Printf("Error is Notfound: %t", errors.Is(err, rabbitmqClient.ErrActionNotfound))
	log.Printf("Error is Timeout: %t", errors.Is(err, rabbitmqClient.ErrRequestTimeout))
}
