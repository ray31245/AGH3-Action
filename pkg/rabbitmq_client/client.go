package rabbitmqclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func init() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		for _, f := range gracefulShutdownList {
			f()
		}
	}()
}

var gracefulShutdownList []func()

func gracefulShutdown(f func()) {
	gracefulShutdownList = append(gracefulShutdownList, f)
}

type RabbitmqClient struct {
	conn    *amqp.Connection
	Channel *amqp.Channel
}

type Exchange struct {
	Name       string
	Type       string
	Durable    bool
	AutoDelete bool
	Internal   bool
	NoWait     bool
	Arguments  amqp.Table
}

func New(url string) (RabbitmqClient, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return RabbitmqClient{}, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}
	gracefulShutdown(func() {
		if err := conn.Close(); err != nil {
			log.Println(err)
		}
	})

	ch, err := conn.Channel()
	if err != nil {
		return RabbitmqClient{}, fmt.Errorf("failed to open a channel: %w", err)
	}
	gracefulShutdown(func() {
		if err := ch.Close(); err != nil {
			log.Println(err)
		}
	})

	return RabbitmqClient{conn: conn, Channel: ch}, nil
}

func (c RabbitmqClient) ForkClient() (RabbitmqClient, error) {
	ch, err := c.conn.Channel()
	if err != nil {
		return RabbitmqClient{}, fmt.Errorf("failed to open a channel: %w", err)
	}
	gracefulShutdown(func() { err := ch.Close(); log.Println(err) })

	return RabbitmqClient{conn: c.conn, Channel: ch}, nil
}

func (client RabbitmqClient) DeclareQueueRpcActionOperate() (amqp.Queue, error) {
	q, err := client.Channel.QueueDeclare(
		"rpc_action_operate", // name
		true,                 // durable
		false,                // delete when unused
		false,                // exclusive
		false,                // no-wait
		nil,                  // arguments
	)
	if err != nil {
		return amqp.Queue{}, fmt.Errorf("fail to declare a queue: %w", err)
	}
	return q, nil
}

var ActionResultExchange = Exchange{
	Name:       "action_result",
	Type:       "direct",
	Durable:    true,
	AutoDelete: false,
	Internal:   false,
	NoWait:     false,
	Arguments:  nil,
}

func (client RabbitmqClient) DeclareExchangeActionResult() error {
	err := client.Channel.ExchangeDeclare(
		ActionResultExchange.Name,
		ActionResultExchange.Type,
		ActionResultExchange.Durable,
		ActionResultExchange.AutoDelete,
		ActionResultExchange.Internal,
		ActionResultExchange.NoWait,
		ActionResultExchange.Arguments,
	)
	return err
}

func (client RabbitmqClient) DeclareQueueLaunchAction() (amqp.Queue, error) {
	q, err := client.Channel.QueueDeclare(
		"launch_action",                 // name
		true,                            // durable
		false,                           // delete when unused
		false,                           // exclusive
		false,                           // no-wait
		amqp.Table{"x-max-priority": 9}, // arguments
	)
	if err != nil {
		return amqp.Queue{}, fmt.Errorf("fail to declare a queue: %w", err)
	}
	return q, nil
}

func (client RabbitmqClient) DeclareTemporaryQueueRpcActionOperate() (amqp.Queue, error) {
	q, err := client.Channel.QueueDeclare(
		"",    // name
		true,  // durable
		false, // delete when unused
		true,  // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return amqp.Queue{}, fmt.Errorf("DeclareTemporaryQueueRpcActionOperate: %w", err)
	}
	return q, nil
}

func (client RabbitmqClient) DeclareTemporaryQueueActionResult() (amqp.Queue, error) {
	q, err := client.Channel.QueueDeclare(
		"",    // name
		true,  // durable
		false, // delete when unused
		true,  // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return amqp.Queue{}, fmt.Errorf("DeclareTemporaryQueueActionResult: %w", err)
	}
	return q, nil
}

func (client RabbitmqClient) ConsumerTemporaryQueueRpcActionOperate(q amqp.Queue) (<-chan amqp.Delivery, error) {
	msgs, err := client.Channel.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		return msgs, fmt.Errorf("ConsumerQueueRpcActionOperate: %w", err)
	}
	return msgs, nil
}

type operate string

const (
	OperateCreate         operate = "createAction"
	OperateGet            operate = "getAction"
	OperateGetByHistoryID operate = "getActionByHistoryID"
	OperateWatchLog       operate = "watchActionLog"
	OperateUpdate         operate = "updateAction"
	OperateDelete         operate = "deleteAction"
	OperateStop           operate = "stopAction"
)

type ActionStatus string

const (
	// Action not run yet
	ActiveStatusPending ActionStatus = "Pending"
	// Action is running
	ActionStatusRuning ActionStatus = "Runing"
	// Action is Successed
	ActionStatusSuccessed ActionStatus = "Successed"
	// Action is Fail
	ActionStatusFail ActionStatus = "Fail"
	// Action is Stoped
	ActionStatusStop ActionStatus = "Stop"
)

type ActionOperateMessageRequest struct {
	Operate operate `json:"operate"`
	Content string  `json:"content"`
}

type ActionModel struct {
	NameSpace string   `json:"nameSpace"`
	Name      string   `json:"name"`
	HistoryID string   `json:"historyID"`
	Image     string   `json:"image"`
	Args      []string `json:"args"`
}

type SelectOne struct {
	NameSpace string `json:"nameSpace"`
	Name      string `json:"name"`
}

type CreateActionReqContent struct {
	Action ActionModel `json:"action"`
}

type GetActionReqContent struct {
	Selector SelectOne `json:"selector"`
}

type GetActionResContent struct {
	Action ActionModel  `json:"action"`
	Status ActionStatus `json:"status"`
}

type GetActionByHistoryReqContent struct {
	HistoryID string `json:"historyID"`
}

type GetActionByHistoryResContent struct {
	ActionList []GetActionResContent `json:"actionList"`
}

type UpdateActionReqContent struct {
	Selector SelectOne   `json:"selector"`
	Action   ActionModel `json:"action"`
}

type DeleteActionReqContent struct {
	Selector SelectOne `json:"selector"`
}

type StopActionReqContent struct {
	Selector SelectOne `json:"selector"`
}

type WatchActionLogReqContent struct {
	Selector SelectOne `json:"selector"`
}

type LaunchActionRequest struct {
	Selector SelectOne `json:"selector"`
}

type WatchStatus string

const (
	WatchStatusStart   = "start"
	WatchStatusSending = "sending"
	WatchStatusClose   = "close"
)

type WatchActionLogResContent struct {
	WatchStatus WatchStatus `json:"watchStatus"`
	Log         string      `json:"log"`
}

type ActionResultMessage struct {
	Action ActionModel  `json:"action"`
	Status ActionStatus `json:"status"`
}

type ActionMessageResponse struct {
	ErrorMsg   string      `json:"errorMsg"`
	ErrorCodes []ErrorCode `json:"errorCodes"`
	Content    string      `json:"content"`
}

func (client RabbitmqClient) ResponseErrorMessage(d amqp.Delivery, errMsg string, code ...ErrorCode) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	response, err := json.Marshal(ActionMessageResponse{ErrorMsg: errMsg, ErrorCodes: code})
	if err != nil {
		response = []byte(err.Error())
	}
	err = client.Channel.PublishWithContext(ctx,
		"",        // exchange
		d.ReplyTo, // routing key
		false,     // mandatory
		false,     // immediate
		amqp.Publishing{
			ContentType:   "text/plain",
			DeliveryMode:  amqp.Persistent,
			CorrelationId: d.CorrelationId,
			Body:          response,
		})
	if err != nil {
		log.Println("failed publish message")
	}
}

func (client RabbitmqClient) ResponseMessage(d amqp.Delivery, content string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	response, err := json.Marshal(ActionMessageResponse{Content: content})
	if err != nil {
		response = []byte(err.Error())
	}
	err = client.Channel.PublishWithContext(ctx,
		"",        // exchange
		d.ReplyTo, // routing key
		false,     // mandatory
		false,     // immediate
		amqp.Publishing{
			ContentType:   "text/plain",
			DeliveryMode:  amqp.Persistent,
			CorrelationId: d.CorrelationId,
			Body:          response,
		})
	if err != nil {
		log.Println("failed publish message")
	}
}

func (client RabbitmqClient) RequestRpcActionOperate(rpc_queue amqp.Queue, temporaryQueue amqp.Queue, corrId string, body []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := client.Channel.PublishWithContext(ctx,
		"",             // exchange
		rpc_queue.Name, // routing key
		false,          // mandatory
		false,          // immediate
		amqp.Publishing{
			ContentType:   "text/plain",
			DeliveryMode:  amqp.Persistent,
			CorrelationId: corrId,
			ReplyTo:       temporaryQueue.Name,
			Body:          body,
			Expiration:    expireTimeStr,
		})
	if err != nil {
		return fmt.Errorf("RequestRpcActionOperate: %w", err)
	}
	return nil
}

const (
	SystemPriority = 0
	UserPriority   = 1
)

func (client RabbitmqClient) RequestLaunchActionByUser(launchActionQueue amqp.Queue, temporaryQueue amqp.Queue, corrId string, body []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := client.Channel.PublishWithContext(ctx,
		"",                     // exchange
		launchActionQueue.Name, // routing key
		false,                  // mandatory
		false,                  // immediate
		amqp.Publishing{
			ContentType:   "text/plain",
			DeliveryMode:  amqp.Persistent,
			CorrelationId: corrId,
			ReplyTo:       temporaryQueue.Name,
			Body:          body,
			Expiration:    expireTimeStr,
			Priority:      UserPriority,
		})
	if err != nil {
		return fmt.Errorf("RequestRpcActionOperate: %w", err)
	}
	return nil
}

func (client RabbitmqClient) RequestLaunchActionBySystem(launchActionQueue amqp.Queue, body []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := client.Channel.PublishWithContext(ctx,
		"",                     // exchange
		launchActionQueue.Name, // routing key
		false,                  // mandatory
		false,                  // immediate
		amqp.Publishing{
			ContentType:  "text/plain",
			DeliveryMode: amqp.Persistent,
			Body:         body,
			Priority:     SystemPriority,
		})
	if err != nil {
		return fmt.Errorf("RequestRpcActionOperate: %w", err)
	}
	return nil
}

func (client RabbitmqClient) WatchActionLogResponse(d amqp.Delivery, watchStatus WatchStatus, log string) error {
	resContent := WatchActionLogResContent{
		WatchStatus: watchStatus,
		Log:         log,
	}
	res, err := json.Marshal(resContent)
	if err != nil {
		return fmt.Errorf("RabbitmqClient.watchActionLogResponse: %w: %w", ErrMarshalResponseContent, err)
	}
	client.ResponseMessage(d, string(res))
	return nil
}

func (client RabbitmqClient) ReceivesRpcActionOperate(msgs <-chan amqp.Delivery, corrId string, extractF func(ActionMessageResponse) error) error {
	for {
		select {
		case d := <-msgs:
			if corrId == d.CorrelationId {
				response := ActionMessageResponse{}
				err := json.Unmarshal(d.Body, &response)
				if err != nil {
					return RpcError{msg: fmt.Errorf("RabbitmqClient.ReceivesRpcActionOperate: %w: %w", ErrUnmarshalResponse, err).Error(), errs: []error{ErrUnmarshalResponse, err}}
				}
				hasErr, err := response.extractError()
				if hasErr {
					return err
				}
				err = extractF(response)
				if err != nil {
					return err
				}
				return nil
			}
		case <-time.After(expireTime):
			return RpcError{msg: fmt.Errorf("RabbitmqClient.ReceivesRpcActionOperate: %s and %s", ErrRetry, ErrRequestTimeout).Error(), errs: []error{ErrRetry, ErrRequestTimeout}}
		}
	}
}
