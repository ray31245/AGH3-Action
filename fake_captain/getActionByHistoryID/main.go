package main

import (
	"fmt"
	"log"
	"sort"
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

	for i := 0; i < 1000; i++ {
		s := ""
		fmt.Printf("\r")
		// fmt.Printf("\n")
		res, err := client.RPC().GetActionByHistoryID(rabbitmqClient.GetActionByHistoryIDRequest{HistoryID: "aaa"})
		failOnError(err, "Failed to GetActionByHistoryID")
		sort.Slice(res.ActionsList, func(i, j int) bool {
			// defer func() {
			// 	fmt.Println("-----")
			// }()
			ni := res.ActionsList[i].Action.Name
			nj := res.ActionsList[j].Action.Name

			max := 0
			if len(ni) > len(nj) {
				max = len(nj)
			} else {
				max = len(ni)
			}

			index := 0
			for index < max {
				// fmt.Println(string(ni[index]), string(nj[index]))
				// fmt.Println(rune(ni[index]) > rune(nj[index]))
				if rune(ni[index]) < rune(nj[index]) {
					// fmt.Println("return")
					return true
				}
				index++
			}
			return len(ni) < len(nj)
		})
		for _, v := range res.ActionsList {
			fmt.Printf("%s: %s ", v.Action.Name, v.Status)
		}
		// b, err := json.Marshal(res)
		// failOnError(err, "Failed to Marshal GetActionByHistoryID")
		// s = string(b)
		// fmt.Printf("%s", s)

		// j := 0
		// for j <= i {
		// 	j++
		// 	if i%2 == 0 {
		// 		s += "#"
		// 	} else {
		// 		s += "%"
		// 	}
		// }
		// if i%2 == 0 {
		// 	s = "aaa"
		// } else {
		// 	s = "qqq"
		// }
		fmt.Printf("%s", s)
		time.Sleep(time.Millisecond * 500)
	}
}
