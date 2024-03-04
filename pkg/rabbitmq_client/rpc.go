package rabbitmqclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RPC struct {
	client RabbitmqClient
}

func (client RabbitmqClient) RPC() RPC {
	return RPC{client: client}
}

var (
	ActionOperateRPCQueueLimit     int = 100
	LaunchActionQueueBySystemLimit int = 1000
	LaunchActionQueueByUserLimit   int = LaunchActionQueueBySystemLimit + 100
	expireTimeInt                  int = 3000 // Millisecond
	expireTimeStr                      = strconv.Itoa(expireTimeInt)
	expireTime                         = time.Millisecond * (time.Duration(expireTimeInt) + 500)
)

func SeedCorrelationId() string {
	newRand := rand.New(rand.NewSource(time.Now().UTC().UnixNano()))
	randInt := func(min int, max int) int {
		return min + newRand.Intn(max-min)
	}
	l := 32
	bytes := make([]byte, l)
	for i := 0; i < l; i++ {
		bytes[i] = byte(randInt(65, 90))
	}
	return string(bytes)

}

type CreateActionRequest struct {
	Action ActionModel `json:"action"`
}

func (r RPC) CreateAction(req CreateActionRequest) error {
	forkClient, err := r.client.ForkClient()
	if err != nil {
		return RpcError{msg: fmt.Errorf("GetActionByHistoryID: %w", err).Error(), errs: []error{err}}
	}
	defer func() {
		if err := forkClient.Channel.Close(); err != nil {
			log.Println(err)
		}
	}()

	// generate a temporary queue
	temporaryQueue, err := forkClient.DeclareTemporaryQueueRpcActionOperate()
	if err != nil {
		return RpcError{msg: fmt.Errorf("CreateAction: %w: %w", ErrCreateTemporaryQueue, err).Error(), errs: []error{ErrCreateTemporaryQueue, err}}
	}
	defer func() {
		if _, err := forkClient.Channel.QueueDelete(temporaryQueue.Name, false, true, false); err != nil {
			log.Println(err)
		}
	}()

	msgs, err := forkClient.ConsumerTemporaryQueueRpcActionOperate(temporaryQueue)
	if err != nil {
		return RpcError{msg: fmt.Errorf("CreateAction: %w: %w", ErrrCreateConsumer, err).Error(), errs: []error{ErrrCreateConsumer, err}}
	}

	rpc_queue, err := forkClient.DeclareQueueRpcActionOperate()
	if err != nil {
		return RpcError{msg: fmt.Errorf("CreateAction: %w: %w", ErrCreateActionOperateRPCQueue, err).Error(), errs: []error{ErrCreateActionOperateRPCQueue, err}}
	}

	if rpc_queue.Messages > ActionOperateRPCQueueLimit {
		return RpcError{msg: fmt.Errorf("CreateAction: %w", ErrRetry).Error(), errs: []error{ErrRetry}}
	}

	content := CreateActionReqContent(req)
	contentStr, err := json.Marshal(content)
	if err != nil {
		return RpcError{msg: fmt.Errorf("CreateAction: %w: %w", ErrMarshalRequestContent, err).Error(), errs: []error{ErrMarshalRequestContent, err}}
	}
	body := ActionOperateMessageRequest{
		Operate: OperateCreate,
		Content: string(contentStr),
	}
	reqByte, err := json.Marshal(body)
	if err != nil {
		return RpcError{msg: fmt.Errorf("CreateAction: %w: %w", ErrMarshalRequest, err).Error(), errs: []error{ErrMarshalRequest, err}}
	}

	corrId := SeedCorrelationId()
	err = forkClient.RequestRpcActionOperate(rpc_queue, temporaryQueue, corrId, reqByte)
	if err != nil {
		return RpcError{msg: fmt.Errorf("CreateAction: %s: %s", ErrPublishMessage, err).Error(), errs: []error{ErrPublishMessage, err}}
	}

	extractF := func(response ActionMessageResponse) error { return nil }
	err = forkClient.ReceivesRpcActionOperate(msgs, corrId, extractF)
	if err != nil {
		return fmt.Errorf("CreateAction: %w", err)
	}
	return nil
}

type GetActionRequest struct {
	Selector SelectOne `json:"selector"`
}

type GetActionResponse struct {
	Action ActionModel  `json:"action"`
	Status ActionStatus `json:"status"`
}

func (r RPC) GetAction(req GetActionRequest) (GetActionResponse, error) {
	res := GetActionResponse{}
	forkClient, err := r.client.ForkClient()
	if err != nil {
		return res, RpcError{msg: fmt.Errorf("GetActionByHistoryID: %w", err).Error(), errs: []error{err}}
	}
	defer func() {
		if err := forkClient.Channel.Close(); err != nil {
			log.Println(err)
		}
	}()

	// generate a temporary queue
	temporaryQueue, err := forkClient.DeclareTemporaryQueueRpcActionOperate()
	if err != nil {
		return res, RpcError{msg: fmt.Errorf("GetAction: %w: %w", ErrCreateTemporaryQueue, err).Error(), errs: []error{ErrCreateTemporaryQueue, err}}
	}
	defer func() {
		if _, err := forkClient.Channel.QueueDelete(temporaryQueue.Name, false, true, false); err != nil {
			log.Println(err)
		}
	}()

	msgs, err := forkClient.ConsumerTemporaryQueueRpcActionOperate(temporaryQueue)
	if err != nil {
		return res, RpcError{msg: fmt.Errorf("GetAction: %w: %w", ErrrCreateConsumer, err).Error(), errs: []error{ErrrCreateConsumer, err}}
	}

	rpc_queue, err := forkClient.DeclareQueueRpcActionOperate()
	if err != nil {
		return res, RpcError{msg: fmt.Errorf("GetAction: %w", err).Error(), errs: []error{err}}
	}

	if rpc_queue.Messages > ActionOperateRPCQueueLimit {
		return res, RpcError{msg: fmt.Errorf("GetAction: %w", ErrRetry).Error(), errs: []error{ErrRetry}}
	}

	content := GetActionReqContent(req)
	contentStr, err := json.Marshal(content)
	if err != nil {
		return res, RpcError{msg: fmt.Errorf("GetAction: %w: %w", ErrMarshalRequestContent, err).Error(), errs: []error{ErrMarshalRequestContent, err}}
	}
	body := ActionOperateMessageRequest{
		Operate: OperateGet,
		Content: string(contentStr),
	}
	reqByte, err := json.Marshal(body)
	if err != nil {
		return res, RpcError{msg: fmt.Errorf("GetAction: %w: %w", ErrMarshalRequest, err).Error(), errs: []error{ErrMarshalRequest, err}}
	}

	corrId := SeedCorrelationId()
	err = forkClient.RequestRpcActionOperate(rpc_queue, temporaryQueue, corrId, reqByte)
	if err != nil {
		return res, RpcError{msg: fmt.Errorf("GetAction: %s: %s", ErrPublishMessage, err).Error(), errs: []error{ErrPublishMessage, err}}
	}

	resContent := GetActionResContent{}
	extractF := func(response ActionMessageResponse) error {
		err := json.Unmarshal([]byte(response.Content), &resContent)
		if err != nil {
			return err
		}
		return nil
	}
	err = forkClient.ReceivesRpcActionOperate(msgs, corrId, extractF)
	if err != nil {
		return res, fmt.Errorf("GetAction: %w", err)
	}
	res = GetActionResponse(resContent)
	return res, nil
}

type GetActionByHistoryIDRequest struct {
	HistoryID string `json:"historyID"`
}

type GetActionByHistoryIDResponse struct {
	ActionsList []GetActionResponse `json:"actionList"`
}

func (r RPC) GetActionByHistoryID(req GetActionByHistoryIDRequest) (GetActionByHistoryIDResponse, error) {
	res := GetActionByHistoryIDResponse{}
	forkClient, err := r.client.ForkClient()
	if err != nil {
		return res, RpcError{msg: fmt.Errorf("GetActionByHistoryID: %w", err).Error(), errs: []error{err}}
	}
	defer func() {
		if err := forkClient.Channel.Close(); err != nil {
			log.Println(err)
		}
	}()

	// generate a temporary queue
	temporaryQueue, err := forkClient.DeclareTemporaryQueueRpcActionOperate()
	if err != nil {
		return res, RpcError{msg: fmt.Errorf("GetActionByHistoryID: %w: %w", ErrCreateTemporaryQueue, err).Error(), errs: []error{ErrCreateTemporaryQueue, err}}
	}
	defer func() {
		if _, err := forkClient.Channel.QueueDelete(temporaryQueue.Name, false, true, false); err != nil {
			log.Println(err)
		}
	}()

	msgs, err := forkClient.ConsumerTemporaryQueueRpcActionOperate(temporaryQueue)
	if err != nil {
		return res, RpcError{msg: fmt.Errorf("GetActionByHistoryID: %w: %w", ErrrCreateConsumer, err).Error(), errs: []error{ErrrCreateConsumer, err}}
	}

	rpc_queue, err := forkClient.DeclareQueueRpcActionOperate()
	if err != nil {
		return res, RpcError{msg: fmt.Errorf("GetActionByHistoryID: %w", err).Error(), errs: []error{err}}
	}

	if rpc_queue.Messages > ActionOperateRPCQueueLimit {
		return res, RpcError{msg: fmt.Errorf("GetActionByHistoryID: %w", ErrRetry).Error(), errs: []error{ErrRetry}}
	}

	content := GetActionByHistoryReqContent(req)
	contentStr, err := json.Marshal(content)
	if err != nil {
		return res, RpcError{msg: fmt.Errorf("GetActionByHistoryID: %w: %w", ErrMarshalRequestContent, err).Error(), errs: []error{ErrMarshalRequestContent, err}}
	}
	body := ActionOperateMessageRequest{
		Operate: OperateGetByHistoryID,
		Content: string(contentStr),
	}
	reqByte, err := json.Marshal(body)
	if err != nil {
		return res, RpcError{msg: fmt.Errorf("GetActionByHistoryID: %w: %w", ErrMarshalRequest, err).Error(), errs: []error{ErrMarshalRequest, err}}
	}

	corrId := SeedCorrelationId()
	err = forkClient.RequestRpcActionOperate(rpc_queue, temporaryQueue, corrId, reqByte)
	if err != nil {
		return res, RpcError{msg: fmt.Errorf("GetActionByHistoryID: %s: %s", ErrPublishMessage, err).Error(), errs: []error{ErrPublishMessage, err}}
	}

	resContent := GetActionByHistoryResContent{}
	extractF := func(response ActionMessageResponse) error {
		err := json.Unmarshal([]byte(response.Content), &resContent)
		if err != nil {
			return err
		}
		return nil
	}
	err = forkClient.ReceivesRpcActionOperate(msgs, corrId, extractF)
	if err != nil {
		return res, fmt.Errorf("GetActionByHistoryID: %w", err)
	}
	res = GetActionByHistoryIDResponse{ActionsList: []GetActionResponse{}}
	for _, v := range resContent.ActionList {
		res.ActionsList = append(res.ActionsList, GetActionResponse(v))
	}
	return res, nil
}

type UpdateActionRequest struct {
	Selector SelectOne   `json:"selector"`
	Action   ActionModel `json:"action"`
}

func (r RPC) UpdateAction(req UpdateActionRequest) error {
	forkClient, err := r.client.ForkClient()
	if err != nil {
		return RpcError{msg: fmt.Errorf("UpdateAction: %w", err).Error(), errs: []error{err}}
	}
	defer func() {
		if err := forkClient.Channel.Close(); err != nil {
			log.Println(err)
		}
	}()
	// generate a temporary queue
	temporaryQueue, err := forkClient.DeclareTemporaryQueueRpcActionOperate()
	if err != nil {
		return RpcError{msg: fmt.Errorf("UpdateAction: %w: %w", ErrCreateTemporaryQueue, err).Error(), errs: []error{ErrCreateTemporaryQueue, err}}
	}
	defer func() {
		if _, err := forkClient.Channel.QueueDelete(temporaryQueue.Name, false, true, false); err != nil {
			log.Println(err)
		}
	}()

	msgs, err := forkClient.ConsumerTemporaryQueueRpcActionOperate(temporaryQueue)
	if err != nil {
		return RpcError{msg: fmt.Errorf("UpdateAction: %w: %w", ErrrCreateConsumer, err).Error(), errs: []error{ErrrCreateConsumer, err}}
	}

	rpc_queue, err := forkClient.DeclareQueueRpcActionOperate()
	if err != nil {
		return RpcError{msg: fmt.Errorf("UpdateAction: %w", err).Error(), errs: []error{err}}
	}

	if rpc_queue.Messages > ActionOperateRPCQueueLimit {
		return RpcError{msg: fmt.Errorf("UpdateAction: %w", ErrRetry).Error(), errs: []error{ErrRetry}}
	}

	content := UpdateActionReqContent(req)
	contentStr, err := json.Marshal(content)
	if err != nil {
		return RpcError{msg: fmt.Errorf("UpdateAction: %w: %w", ErrMarshalRequestContent, err).Error(), errs: []error{ErrMarshalRequestContent, err}}
	}
	body := ActionOperateMessageRequest{
		Operate: OperateUpdate,
		Content: string(contentStr),
	}
	reqByte, err := json.Marshal(body)
	if err != nil {
		return RpcError{msg: fmt.Errorf("UpdateAction: %w: %w", ErrMarshalRequest, err).Error(), errs: []error{ErrMarshalRequest, err}}
	}

	corrId := SeedCorrelationId()
	err = forkClient.RequestRpcActionOperate(rpc_queue, temporaryQueue, corrId, reqByte)
	if err != nil {
		return RpcError{msg: fmt.Errorf("UpdateAction: %s: %s", ErrPublishMessage, err).Error(), errs: []error{ErrPublishMessage, err}}
	}

	extractF := func(response ActionMessageResponse) error { return nil }
	err = forkClient.ReceivesRpcActionOperate(msgs, corrId, extractF)
	if err != nil {
		return fmt.Errorf("UpdateAction: %w", err)
	}
	return nil
}

type DeleteActionRequest struct {
	Selector SelectOne `json:"selector"`
}

func (r RPC) DeleteAction(req DeleteActionRequest) error {
	forkClient, err := r.client.ForkClient()
	if err != nil {
		return RpcError{msg: fmt.Errorf("DeleteAction: %w", err).Error(), errs: []error{err}}
	}
	defer func() {
		if err := forkClient.Channel.Close(); err != nil {
			log.Println(err)
		}
	}()

	// generate a temporary queue
	temporaryQueue, err := forkClient.DeclareTemporaryQueueRpcActionOperate()
	if err != nil {
		return RpcError{msg: fmt.Errorf("DeleteAction: %w: %w", ErrCreateTemporaryQueue, err).Error(), errs: []error{ErrCreateTemporaryQueue, err}}
	}
	defer func() {
		if _, err := forkClient.Channel.QueueDelete(temporaryQueue.Name, false, true, false); err != nil {
			log.Println(err)
		}
	}()

	msgs, err := forkClient.ConsumerTemporaryQueueRpcActionOperate(temporaryQueue)
	if err != nil {
		return RpcError{msg: fmt.Errorf("DeleteAction: %w: %w", ErrrCreateConsumer, err).Error(), errs: []error{ErrrCreateConsumer, err}}
	}

	rpc_queue, err := forkClient.DeclareQueueRpcActionOperate()
	if err != nil {
		return RpcError{msg: fmt.Errorf("DeleteAction: %w", err).Error(), errs: []error{err}}
	}

	if rpc_queue.Messages > ActionOperateRPCQueueLimit {
		return RpcError{msg: fmt.Errorf("DeleteAction: %w", ErrRetry).Error(), errs: []error{ErrRetry}}
	}

	content := DeleteActionReqContent(req)
	contentStr, err := json.Marshal(content)
	if err != nil {
		return RpcError{msg: fmt.Errorf("DeleteAction: %w: %w", ErrMarshalRequestContent, err).Error(), errs: []error{ErrMarshalRequestContent, err}}
	}
	body := ActionOperateMessageRequest{
		Operate: OperateDelete,
		Content: string(contentStr),
	}
	reqByte, err := json.Marshal(body)
	if err != nil {
		return RpcError{msg: fmt.Errorf("DeleteAction: %w: %w", ErrMarshalRequest, err).Error(), errs: []error{ErrMarshalRequest, err}}
	}

	corrId := SeedCorrelationId()
	err = forkClient.RequestRpcActionOperate(rpc_queue, temporaryQueue, corrId, reqByte)
	if err != nil {
		return RpcError{msg: fmt.Errorf("DeleteAction: %s: %s", ErrPublishMessage, err).Error(), errs: []error{ErrPublishMessage, err}}
	}

	extractF := func(response ActionMessageResponse) error { return nil }
	err = forkClient.ReceivesRpcActionOperate(msgs, corrId, extractF)
	if err != nil {
		return fmt.Errorf("DeleteAction: %w", err)
	}
	return nil
}

type StopActionRequest struct {
	Selector SelectOne `json:"selector"`
}

func (r RPC) StopAction(req StopActionRequest) error {
	forkClient, err := r.client.ForkClient()
	if err != nil {
		return RpcError{msg: fmt.Errorf("StopAction: %w", err).Error(), errs: []error{err}}
	}
	defer func() {
		if err := forkClient.Channel.Close(); err != nil {
			log.Println(err)
		}
	}()

	// generate a temporary queue
	temporaryQueue, err := forkClient.DeclareTemporaryQueueRpcActionOperate()
	if err != nil {
		return RpcError{msg: fmt.Errorf("StopAction: %w: %w", ErrCreateTemporaryQueue, err).Error(), errs: []error{ErrCreateTemporaryQueue, err}}
	}
	defer func() {
		if _, err := forkClient.Channel.QueueDelete(temporaryQueue.Name, false, true, false); err != nil {
			log.Println(err)
		}
	}()

	msgs, err := forkClient.ConsumerTemporaryQueueRpcActionOperate(temporaryQueue)
	if err != nil {
		return RpcError{msg: fmt.Errorf("StopAction: %w: %w", ErrrCreateConsumer, err).Error(), errs: []error{ErrrCreateConsumer, err}}
	}

	rpc_queue, err := forkClient.DeclareQueueRpcActionOperate()
	if err != nil {
		return RpcError{msg: fmt.Errorf("StopAction: %w", err).Error(), errs: []error{err}}
	}

	if rpc_queue.Messages > ActionOperateRPCQueueLimit {
		return RpcError{msg: fmt.Errorf("StopAction: %w", ErrRetry).Error(), errs: []error{ErrRetry}}
	}

	content := StopActionReqContent(req)
	contentStr, err := json.Marshal(content)
	if err != nil {
		return RpcError{msg: fmt.Errorf("StopAction: %w: %w", ErrMarshalRequestContent, err).Error(), errs: []error{ErrMarshalRequestContent, err}}
	}
	body := ActionOperateMessageRequest{
		Operate: OperateStop,
		Content: string(contentStr),
	}
	reqByte, err := json.Marshal(body)
	if err != nil {
		return RpcError{msg: fmt.Errorf("StopAction: %w: %w", ErrMarshalRequest, err).Error(), errs: []error{ErrMarshalRequest, err}}
	}

	corrId := SeedCorrelationId()
	err = forkClient.RequestRpcActionOperate(rpc_queue, temporaryQueue, corrId, reqByte)
	if err != nil {
		return RpcError{msg: fmt.Errorf("StopAction: %s: %s", ErrPublishMessage, err).Error(), errs: []error{ErrPublishMessage, err}}
	}

	extractF := func(response ActionMessageResponse) error { return nil }
	err = forkClient.ReceivesRpcActionOperate(msgs, corrId, extractF)
	if err != nil {
		return fmt.Errorf("StopAction: %w", err)
	}
	return nil
}

type WatchActionLogRequest struct {
	Selector SelectOne `json:"selector"`
}

type WatchActionLogResponse string

func (r RPC) WatchActionLog(ctx context.Context, req WatchActionLogRequest) (<-chan WatchActionLogResponse, error) {
	errCh := make(chan error, 20)
	logCh := make(chan WatchActionLogResponse)
	go func() {
		defer close(logCh)
		forkClient, err := r.client.ForkClient()
		if err != nil {
			errCh <- RpcError{msg: fmt.Errorf("StopAction: %w", err).Error(), errs: []error{err}}
			return
		}
		defer func() {
			if err := forkClient.Channel.Close(); err != nil {
				log.Println(err)
			}
		}()

		// generate a temporary queue
		temporaryQueue, err := r.client.DeclareTemporaryQueueRpcActionOperate()
		if err != nil {
			errCh <- RpcError{msg: fmt.Errorf("UpdateAction: %w: %w", ErrCreateTemporaryQueue, err).Error(), errs: []error{ErrCreateTemporaryQueue, err}}
			return
		}
		defer func() {
			if _, err := forkClient.Channel.QueueDelete(temporaryQueue.Name, false, true, false); err != nil {
				log.Println(err)
			}
		}()

		msgs, err := r.client.ConsumerTemporaryQueueRpcActionOperate(temporaryQueue)
		if err != nil {
			errCh <- RpcError{msg: fmt.Errorf("UpdateAction: %w: %w", ErrrCreateConsumer, err).Error(), errs: []error{ErrrCreateConsumer, err}}
			return
		}

		rpc_queue, err := r.client.DeclareQueueRpcActionOperate()
		if err != nil {
			errCh <- RpcError{msg: fmt.Errorf("UpdateAction: %w", err).Error(), errs: []error{err}}
			return
		}

		if rpc_queue.Messages > ActionOperateRPCQueueLimit {
			errCh <- RpcError{msg: fmt.Errorf("UpdateAction: %w", ErrRetry).Error(), errs: []error{ErrRetry}}
			return
		}

		content := WatchActionLogReqContent(req)
		contentStr, err := json.Marshal(content)
		if err != nil {
			errCh <- RpcError{msg: fmt.Errorf("UpdateAction: %w: %w", ErrMarshalRequestContent, err).Error(), errs: []error{ErrMarshalRequestContent, err}}
			return
		}

		body := ActionOperateMessageRequest{
			Operate: OperateWatchLog,
			Content: string(contentStr),
		}
		reqByte, err := json.Marshal(body)
		if err != nil {
			errCh <- RpcError{msg: fmt.Errorf("UpdateAction: %w: %w", ErrMarshalRequest, err).Error(), errs: []error{ErrMarshalRequest, err}}
			return
		}

		corrId := SeedCorrelationId()
		err = r.client.RequestRpcActionOperate(rpc_queue, temporaryQueue, corrId, reqByte)
		if err != nil {
			errCh <- RpcError{msg: fmt.Errorf("UpdateAction: %s: %s", ErrPublishMessage, err).Error(), errs: []error{ErrPublishMessage, err}}
			return
		}

		started := false
		for {
			select {
			case d, ok := <-msgs:
				if !ok {
					return
				}
				if corrId == d.CorrelationId {
					response := ActionMessageResponse{}
					err := json.Unmarshal(d.Body, &response)
					if err != nil {
						errCh <- RpcError{msg: fmt.Errorf("UpdateAction: %w: %w", ErrUnmarshalResponse, err).Error(), errs: []error{ErrUnmarshalResponse, err}}
						return
					}
					hasErr, err := response.extractError()
					if hasErr {
						errCh <- err
						return
					}
					if !started {
						started = true
						errCh <- nil
					}

					res := WatchActionLogResContent{}
					err = json.Unmarshal([]byte(response.Content), &res)
					if err != nil {
						errCh <- err
						return
					}
					if res.WatchStatus == WatchStatusSending {
						logCh <- WatchActionLogResponse(res.Log)
					} else if res.WatchStatus == WatchStatusClose {
						return
					}
				}
			case <-time.After(expireTime):
				if !started {
					errCh <- RpcError{msg: fmt.Errorf("UpdateAction: %s and %s", ErrRetry, ErrRequestTimeout).Error(), errs: []error{ErrRetry, ErrRequestTimeout}}
					return
				}
			case <-ctx.Done():
				return
			}
		}

	}()

	err := <-errCh
	if err != nil {
		return nil, err
	}
	return logCh, nil
}

func (r RPC) ListenOnActionResult(ctx context.Context) (<-chan ActionResultMessage, amqp.Queue, error) {
	forkClient, err := r.client.ForkClient()
	if err != nil {
		return nil, amqp.Queue{}, fmt.Errorf("failed to fork client: %w", err)
	}

	err = forkClient.Channel.Qos(
		1,
		0,
		false,
	)
	if err != nil {
		return nil, amqp.Queue{}, fmt.Errorf("failed to bind queue: %w", err)
	}

	receiveActionResultQueue, err := forkClient.DeclareTemporaryQueueActionResult()
	if err != nil {
		return nil, amqp.Queue{}, fmt.Errorf("failed to declare temporary queue: %w", err)
	}

	err = forkClient.DeclareExchangeActionResult()
	if err != nil {
		return nil, amqp.Queue{}, fmt.Errorf("failed to declare exchange: %w", err)
	}

	msgs, err := forkClient.Channel.Consume(
		receiveActionResultQueue.Name,
		"",
		true,
		true,
		false,
		false,
		nil,
	)
	if err != nil {
		return nil, amqp.Queue{}, fmt.Errorf("failed to declare consumer: %w", err)
	}
	res := make(chan ActionResultMessage)
	go func() {
		defer func() {
			if _, err := forkClient.Channel.QueueDelete(receiveActionResultQueue.Name, false, false, true); err != nil {
				log.Println(err)
			}
			if err := forkClient.Channel.Close(); err != nil {
				log.Println(err)
			}
			close(res)
		}()
		for {
			select {
			case v, ok := <-msgs:
				if !ok {
					return
				}
				resultMsg := ActionResultMessage{}
				err := json.Unmarshal(v.Body, &resultMsg)
				if err != nil {
					log.Println("Unmarshal message to ActionResultMessage")
				}
				res <- resultMsg
			case <-ctx.Done():
				return
			}
		}
	}()
	return res, receiveActionResultQueue, nil
}

type LaunchAction struct {
	client                   RabbitmqClient
	receiveActionResultQueue amqp.Queue
}

func (r RPC) LaunchAction(receiveActionResultQueue amqp.Queue) LaunchAction {
	return LaunchAction{
		client:                   r.client,
		receiveActionResultQueue: receiveActionResultQueue,
	}
}

type UserLaunchActionRequest struct {
	Selector  SelectOne `json:"selector"`
	HistoryID string    `json:"historyID"`
}

func (r LaunchAction) UserLaunchAction(req UserLaunchActionRequest) error {
	if req.HistoryID == "" {
		return errors.New("field HistoryID should not empty")
	}
	forkClient, err := r.client.ForkClient()
	if err != nil {
		return RpcError{msg: fmt.Errorf("UserLaunchAction: %w", err).Error(), errs: []error{err}}
	}
	defer func() {
		if err := forkClient.Channel.Close(); err != nil {
			log.Println(err)
		}
	}()

	err = forkClient.Channel.QueueBind(
		r.receiveActionResultQueue.Name,
		req.HistoryID,
		ActionResultExchange.Name,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("UserLaunchAction: %w", err)
	}

	// generate a temporary queue
	temporaryQueue, err := forkClient.DeclareTemporaryQueueRpcActionOperate()
	if err != nil {
		return RpcError{msg: fmt.Errorf("UserLaunchAction: %w: %w", ErrCreateTemporaryQueue, err).Error(), errs: []error{ErrCreateTemporaryQueue, err}}
	}
	defer func() {
		if _, err := forkClient.Channel.QueueDelete(temporaryQueue.Name, false, true, false); err != nil {
			log.Println(err)
		}
	}()

	msgs, err := forkClient.ConsumerTemporaryQueueRpcActionOperate(temporaryQueue)
	if err != nil {
		return RpcError{msg: fmt.Errorf("UserLaunchAction: %w: %w", ErrrCreateConsumer, err).Error(), errs: []error{ErrrCreateConsumer, err}}
	}

	launchActionQueue, err := forkClient.DeclareQueueLaunchAction()
	if err != nil {
		return RpcError{msg: fmt.Errorf("UserLaunchAction: %w", err).Error(), errs: []error{err}}
	}

	if launchActionQueue.Messages > LaunchActionQueueByUserLimit {
		return RpcError{msg: fmt.Errorf("UserLaunchAction: %w", ErrRetry).Error(), errs: []error{ErrRetry}}
	}

	body := LaunchActionRequest{Selector: req.Selector}
	reqByte, err := json.Marshal(body)
	if err != nil {
		return RpcError{msg: fmt.Errorf("UserLaunchAction: %w: %w", ErrMarshalRequest, err).Error(), errs: []error{ErrMarshalRequest, err}}
	}

	corrId := SeedCorrelationId()
	err = forkClient.RequestLaunchActionByUser(launchActionQueue, temporaryQueue, corrId, reqByte)
	if err != nil {
		return RpcError{msg: fmt.Errorf("UserLaunchAction: %s: %s", ErrPublishMessage, err).Error(), errs: []error{ErrPublishMessage, err}}
	}

	extractF := func(response ActionMessageResponse) error { return nil }
	err = forkClient.ReceivesRpcActionOperate(msgs, corrId, extractF)
	if err != nil {
		return fmt.Errorf("UserLaunchAction: %w", err)
	}
	return nil
}

type SystemLaunchActionRequest struct {
	Selector  SelectOne `json:"selector"`
	HistoryID string    `json:"historyID"`
}

func (r LaunchAction) SystemLaunchAction(req SystemLaunchActionRequest) error {
	if req.HistoryID == "" {
		return errors.New("field HistoryID should not empty")
	}

	forkClient, err := r.client.ForkClient()
	if err != nil {
		return RpcError{msg: fmt.Errorf("SystemLaunchAction: %w", err).Error(), errs: []error{err}}
	}
	defer func() {
		if err := forkClient.Channel.Close(); err != nil {
			log.Println(err)
		}
	}()

	forkClient.Channel.QueueBind(
		r.receiveActionResultQueue.Name,
		req.HistoryID,
		ActionResultExchange.Name,
		false,
		nil,
	)

	launchActionQueue, err := forkClient.DeclareQueueLaunchAction()
	if err != nil {
		return RpcError{msg: fmt.Errorf("UserLaunchAction: %w", err).Error(), errs: []error{err}}
	}

	if launchActionQueue.Messages > LaunchActionQueueBySystemLimit {
		return RpcError{msg: fmt.Errorf("UserLaunchAction: %w", ErrRetry).Error(), errs: []error{ErrRetry}}
	}

	body := LaunchActionRequest{Selector: req.Selector}
	reqByte, err := json.Marshal(body)
	if err != nil {
		return RpcError{msg: fmt.Errorf("UserLaunchAction: %w: %w", ErrMarshalRequest, err).Error(), errs: []error{ErrMarshalRequest, err}}
	}

	err = forkClient.RequestLaunchActionBySystem(launchActionQueue, reqByte)
	if err != nil {
		return RpcError{msg: fmt.Errorf("UserLaunchAction: %s: %s", ErrPublishMessage, err).Error(), errs: []error{ErrPublishMessage, err}}
	}

	return nil
}
