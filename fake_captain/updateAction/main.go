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

	action1 := rabbitmqClient.ActionModel{
		Name:  "foo1",
		Image: "busybox",
		Args:  []string{"env"},
	}
	err = client.RPC().CreateAction(rabbitmqClient.CreateActionRequest{
		Action: action1,
	})
	failOnError(err, "Failed to create action1")

	action2 := rabbitmqClient.ActionModel{
		Name:  "foo2",
		Image: "busybox",
		Args:  []string{"env"},
	}
	err = client.RPC().CreateAction(rabbitmqClient.CreateActionRequest{
		Action: action2,
	})
	failOnError(err, "Failed to create action2")

	// try to update a not exist action, should get "notfound" error
	action0 := rabbitmqClient.ActionModel{Name: "foo0"}
	err = client.RPC().UpdateAction(rabbitmqClient.UpdateActionRequest{
		Selector: rabbitmqClient.SelectOne{Name: action0.Name},
	})
	log.Printf("update a not exist action, get error notfound: %t", errors.Is(err, rabbitmqClient.ErrActionNotfound))
	// update failed, should not change everything
	_, err = client.RPC().GetAction(rabbitmqClient.GetActionRequest{Selector: rabbitmqClient.SelectOne{Name: action1.Name}})
	log.Printf("action1 still here: %t", err == nil)
	_, err = client.RPC().GetAction(rabbitmqClient.GetActionRequest{Selector: rabbitmqClient.SelectOne{Name: action2.Name}})
	log.Printf("action2 still here: %t", err == nil)

	// try to update action2 to a exist action, should get "already" error
	err = client.RPC().UpdateAction(rabbitmqClient.UpdateActionRequest{
		Selector: rabbitmqClient.SelectOne{Name: action2.Name},
		Action:   action1},
	)
	log.Printf("update a action to a already exist action, get error already exist: %t", errors.Is(err, rabbitmqClient.ErrActionAlreadyExist))
	// update failed, should not change everything
	_, err = client.RPC().GetAction(rabbitmqClient.GetActionRequest{Selector: rabbitmqClient.SelectOne{Name: action1.Name}})
	log.Printf("action1 still here: %t", err == nil)
	_, err = client.RPC().GetAction(rabbitmqClient.GetActionRequest{Selector: rabbitmqClient.SelectOne{Name: action2.Name}})
	log.Printf("action2 still here: %t", err == nil)

	// normal update action, should be success
	action3 := rabbitmqClient.ActionModel{
		Name:      "foo3",
		NameSpace: "test",
		Image:     "busybox",
		Args:      []string{"env"},
	}
	err = client.RPC().UpdateAction(rabbitmqClient.UpdateActionRequest{
		Selector: rabbitmqClient.SelectOne{Name: action2.Name},
		Action:   action3,
	})
	failOnError(err, "Failed to update action2")

	res, err := client.RPC().GetAction(rabbitmqClient.GetActionRequest{
		Selector: rabbitmqClient.SelectOne{Name: action3.Name, NameSpace: action3.NameSpace},
	})
	failOnError(err, "Failed to get updated action")
	log.Printf("get updated action: %v", res)
}
