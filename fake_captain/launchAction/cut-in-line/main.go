package main

import (
	"log"
	"strconv"
	"time"

	"github.com/rabbitmq/amqp091-go"
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

	err = client.RPC().LaunchAction(amqp091.Queue{}).SystemLaunchAction(rabbitmqClient.SystemLaunchActionRequest{Selector: rabbitmqClient.SelectOne{Name: action.Name}})
	log.Println(err)
	log.Printf("Success launch action: %t", err == nil)

	count := 50
	actionList := []rabbitmqClient.ActionModel{}
	for i := 1; i < count; i++ {
		a := action
		a.Name += a.Name + strconv.Itoa(i)
		actionList = append(actionList, a)
	}
	for _, v := range actionList {
		err = client.RPC().CreateAction(rabbitmqClient.CreateActionRequest{
			Action: v,
		})
		failOnError(err, "Failed to create action")
	}
	time.Sleep(time.Second * 5)
	for _, v := range actionList {
		err = client.RPC().LaunchAction(amqp091.Queue{}).SystemLaunchAction(rabbitmqClient.SystemLaunchActionRequest{Selector: rabbitmqClient.SelectOne{Name: v.Name}})
		log.Println(err)
		log.Printf("Success launch action: %t", err == nil)
	}
	userLaunchAction := action
	userLaunchAction.Name += "byuser"
	err = client.RPC().CreateAction(rabbitmqClient.CreateActionRequest{
		Action: userLaunchAction,
	})
	failOnError(err, "Failed to create action")

	time.Sleep(time.Second * 5)

	err = client.RPC().LaunchAction(amqp091.Queue{}).UserLaunchAction(rabbitmqClient.UserLaunchActionRequest{Selector: rabbitmqClient.SelectOne{Name: userLaunchAction.Name}})
	log.Println(err)
	log.Printf("Success launch action: %t", err == nil)

	time.Sleep(time.Second * 5)

	getUserLaunch, err := client.RPC().GetAction(rabbitmqClient.GetActionRequest{Selector: rabbitmqClient.SelectOne{Name: userLaunchAction.Name}})
	log.Println(err)
	log.Printf("Get User launch action fail: %t", err == nil)
	log.Printf("Cut in line success: %t", getUserLaunch.Status == rabbitmqClient.ActionStatusRuning)

	// // delete a not exist action, should get a notfound error
	// err = client.RPC().DeleteAction(rabbitmqClient.DeleteActionRequest{Selector: rabbitmqClient.SelectOne{Name: "notfound"}})
	// log.Printf("delete a not exist action, get error notfound: %t", errors.Is(err, rabbitmqClient.ErrActionNotfound))

	// err = client.RPC().StopAction(rabbitmqClient.DeleteActionRequest{Selector: rabbitmqClient.SelectOne{Name: action.Name}})
	// log.Println(err)
	// log.Printf("Success stop action: %t", err == nil)
}
