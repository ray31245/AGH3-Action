package main

import (
	"errors"
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

	err = client.RPC().CreateAction(rabbitmqClient.CreateActionRequest{
		Action: rabbitmqClient.ActionModel{
			Name: "foo",
			Containers: []rabbitmqClient.ContainerModel{
				{
					Image: "busybox",
					Args: []string{
						"ping",
						"127.0.0.1",
						"-c",
						"30",
					},
				},
			},
		},
	})

	log.Printf(" [.] Got %v", err)
	log.Printf("Error is Already Exists: %t", errors.Is(err, rabbitmqClient.ErrActionAlreadyExist))
	log.Printf("Error is Timeout: %t", errors.Is(err, rabbitmqClient.ErrRequestTimeout))
}
