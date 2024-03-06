package main

import (
	"context"
	"log"
	"time"

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
		Name:      "foo",
		HistoryID: "aaa",
		Containers: []rabbitmqClient.ContainerModel{
			{
				Image: "busybox",
				Args: []string{
					"ping",
					"127.0.0.1",
					"-c",
					"600",
				},
				Env:          map[string]string{"aaa": "123"},
				VolumeMounts: map[string]string{"share": "/var/stdio"},
			},
			{
				Image: "busybox",
				Args: []string{
					"sleep",
					"300",
				},
				VolumeMounts: map[string]string{"share": "/var/stdio"},
			},
		},
	}
	action1 := rabbitmqClient.ActionModel{
		Name:      "foo1",
		HistoryID: "aaa",
		Containers: []rabbitmqClient.ContainerModel{
			{
				Image: "busybox",
				Args: []string{
					"ping",
					"127.0.0.1",
					"-c",
					"60",
				},
			},
		},
	}
	action2 := rabbitmqClient.ActionModel{
		Name:      "foo2",
		HistoryID: "aaa",
		Containers: []rabbitmqClient.ContainerModel{
			{
				Image: "busybox",
				Args: []string{
					"env",
				},
			},
		},
	}
	action1_1 := rabbitmqClient.ActionModel{
		Name:      "foo1-1",
		HistoryID: "aaa",
		Containers: []rabbitmqClient.ContainerModel{
			{
				Image: "busybox",
				Args: []string{
					"env",
				},
			},
		},
	}
	action1_2 := rabbitmqClient.ActionModel{
		Name:      "foo1-2",
		HistoryID: "aaa",
		Containers: []rabbitmqClient.ContainerModel{
			{
				Image: "busybox",
				Args: []string{
					"env",
				},
			},
		},
	}
	action2_1 := rabbitmqClient.ActionModel{
		Name:      "foo2-1",
		HistoryID: "aaa",
		Containers: []rabbitmqClient.ContainerModel{
			{
				Image: "busybox",
				Args: []string{
					"env",
				},
			},
		},
	}
	action2_2 := rabbitmqClient.ActionModel{
		Name:      "foo2-2",
		HistoryID: "aaa",
		Containers: []rabbitmqClient.ContainerModel{
			{
				Image: "busybox",
				Args: []string{
					"env",
				},
			},
		},
	}

	nextMap := map[string][]rabbitmqClient.ActionModel{}
	nextMap[action1.Name] = []rabbitmqClient.ActionModel{action1_1, action1_2}
	nextMap[action2.Name] = []rabbitmqClient.ActionModel{action2_1, action2_2}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result, Launcher, err := client.RPC().ListenOnActionResult(ctx)
	failOnError(err, "Failed to start ListenOnActionResult")

	time.Sleep(time.Second * 3)

	err = client.RPC().CreateAction(rabbitmqClient.CreateActionRequest{
		Action: action,
	})
	failOnError(err, "Failed to create action")

	err = client.RPC().CreateAction(rabbitmqClient.CreateActionRequest{
		Action: action1,
	})
	failOnError(err, "Failed to create action")
	err = client.RPC().CreateAction(rabbitmqClient.CreateActionRequest{
		Action: action1_1,
	})
	failOnError(err, "Failed to create action")
	err = client.RPC().CreateAction(rabbitmqClient.CreateActionRequest{
		Action: action1_2,
	})
	failOnError(err, "Failed to create action")
	err = client.RPC().CreateAction(rabbitmqClient.CreateActionRequest{
		Action: action2,
	})
	failOnError(err, "Failed to create action")
	err = client.RPC().CreateAction(rabbitmqClient.CreateActionRequest{
		Action: action2_1,
	})
	failOnError(err, "Failed to create action")
	err = client.RPC().CreateAction(rabbitmqClient.CreateActionRequest{
		Action: action2_2,
	})
	failOnError(err, "Failed to create action")

	time.Sleep(time.Second * 3)

	err = Launcher.UserLaunchAction(
		rabbitmqClient.UserLaunchActionRequest{
			Selector:  rabbitmqClient.SelectOne{Name: action.Name},
			HistoryID: action.HistoryID,
		},
	)
	log.Printf("Success launch action %s: %t", action.Name, err == nil)

	err = Launcher.UserLaunchAction(
		rabbitmqClient.UserLaunchActionRequest{
			Selector:  rabbitmqClient.SelectOne{Name: action1.Name},
			HistoryID: action1.HistoryID,
		},
	)
	log.Printf("Success launch action %s: %t", action1.Name, err == nil)
	err = Launcher.UserLaunchAction(
		rabbitmqClient.UserLaunchActionRequest{
			Selector:  rabbitmqClient.SelectOne{Name: action2.Name},
			HistoryID: action2.HistoryID,
		},
	)
	log.Printf("Success launch action %s: %t", action2.Name, err == nil)

	for v := range result {
		log.Printf("action %s finish in status %s", v.GetAction().Name, v.GetStatus())
		log.Println(v.GetLogs())
		if v.GetStatus() == rabbitmqClient.ActionStatusSuccessed {
			for _, nextAction := range nextMap[v.GetAction().Name] {
				err = Launcher.SystemLaunchAction(
					rabbitmqClient.SystemLaunchActionRequest{
						Selector:  rabbitmqClient.SelectOne{Name: nextAction.Name},
						HistoryID: nextAction.HistoryID,
					},
				)
				log.Printf("Success launch action %s: %t", nextAction.Name, err == nil)
			}
		}
	}
}
