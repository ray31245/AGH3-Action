package main

import (
	"log"

	rabbitmqClient "github.com/Leukocyte-Lab/AGH3-Action/pkg/rabbitmq_client"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}

func main() {
	client, err := rabbitmqClient.New("amqp://guest:guest@localhost:5672/")
	failOnError(err, "Failed to create rabbitmqClient")

	logch, err := client.RPC().WatchActionLog(rabbitmqClient.WatchActionLogRequest{
		Selector: rabbitmqClient.SelectOne{Name: "foo"},
	})
	failOnError(err, "Failed to watch action log")
	for v := range logch {
		log.Println(v)
	}
}
