package main

import (
	"context"
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

	action := rabbitmqClient.ActionModel{
		Name:      "foo",
		HistoryID: "aaa",
		Image:     "busybox",
		Args: []string{
			"ping",
			"127.0.0.1",
			"-c",
			"10",
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result, receiveActionResultQueue, err := client.ListenOnActionResult(ctx)
	failOnError(err, "Failed to start ListenOnActionResult")

	err = client.RPC().CreateAction(rabbitmqClient.CreateActionRequest{
		Action: action,
	})
	failOnError(err, "Failed to create action")

	err = client.RPC().LaunchAction(receiveActionResultQueue).UserLaunchAction(rabbitmqClient.UserLaunchActionRequest{Selector: rabbitmqClient.SelectOne{Name: action.Name}, HistoryID: action.HistoryID})
	log.Println(err)
	log.Printf("Success launch action: %t", err == nil)

	for v := range result {
		log.Println(v)
	}

	// count := 50
	// actionList := []rabbitmqClient.ActionModel{}
	// for i := 1; i < count; i++ {
	// 	a := action
	// 	a.Name += a.Name + strconv.Itoa(i)
	// 	actionList = append(actionList, a)
	// }
	// for _, v := range actionList {
	// 	err = client.RPC().CreateAction(rabbitmqClient.CreateActionRequest{
	// 		Action: v,
	// 	})
	// 	failOnError(err, "Failed to create action")
	// }
	// time.Sleep(time.Second * 10)
	// for _, v := range actionList {
	// 	err = client.RPC().UserLaunchAction(rabbitmqClient.UserLaunchActionRequest{Selector: rabbitmqClient.SelectOne{Name: v.Name}})
	// 	log.Println(err)
	// 	log.Printf("Success launch action: %t", err == nil)
	// }

	// // delete a not exist action, should get a notfound error
	// err = client.RPC().DeleteAction(rabbitmqClient.DeleteActionRequest{Selector: rabbitmqClient.SelectOne{Name: "notfound"}})
	// log.Printf("delete a not exist action, get error notfound: %t", errors.Is(err, rabbitmqClient.ErrActionNotfound))

	// err = client.RPC().StopAction(rabbitmqClient.DeleteActionRequest{Selector: rabbitmqClient.SelectOne{Name: action.Name}})
	// log.Println(err)
	// log.Printf("Success stop action: %t", err == nil)
}
