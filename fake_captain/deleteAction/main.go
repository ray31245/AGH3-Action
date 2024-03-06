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

	action := rabbitmqClient.ActionModel{
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
	}
	err = client.RPC().CreateAction(rabbitmqClient.CreateActionRequest{
		Action: action,
	})
	failOnError(err, "Failed to create action")

	// delete a not exist action, should get a notfound error
	err = client.RPC().DeleteAction(rabbitmqClient.DeleteActionRequest{Selector: rabbitmqClient.SelectOne{Name: "notfound"}})
	log.Printf("delete a not exist action, get error notfound: %t", errors.Is(err, rabbitmqClient.ErrActionNotfound))

	err = client.RPC().DeleteAction(rabbitmqClient.DeleteActionRequest{Selector: rabbitmqClient.SelectOne{Name: action.Name}})
	log.Println(err)
	log.Printf("Success delete action: %t", err == nil)
}
