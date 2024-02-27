package rabbitmqclient

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"time"
)

type RPC struct {
	client RabbitmqClient
}

func (client RabbitmqClient) RPC() RPC {
	return RPC{client: client}
}

var (
	ActionOperateRPCQueueLimit int = 100
	expireTimeInt              int = 3000 // Millisecond
	expireTimeStr                  = strconv.Itoa(expireTimeInt)
	expireTime                     = time.Millisecond * time.Duration(expireTimeInt)
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
	// generate a temporary queue
	temporaryQueue, err := r.client.DeclareTemporaryQueueRpcActionOperate()
	if err != nil {
		return RpcError{msg: fmt.Errorf("CreateAction: %w: %w", ErrCreateTemporaryQueue, err).Error(), errs: []error{ErrCreateTemporaryQueue, err}}
	}

	msgs, err := r.client.ConsumerTemporaryQueueRpcActionOperate(temporaryQueue)
	if err != nil {
		return RpcError{msg: fmt.Errorf("CreateAction: %w: %w", ErrrCreateConsumer, err).Error(), errs: []error{ErrrCreateConsumer, err}}
	}

	rpc_queue, err := r.client.DeclareQueueRpcActionOperate()
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
	err = r.client.RequestRpcActionOperate(rpc_queue, temporaryQueue, corrId, reqByte)
	if err != nil {
		return RpcError{msg: fmt.Errorf("CreateAction: %s: %s", ErrPublishMessage, err).Error(), errs: []error{ErrPublishMessage, err}}
	}

	extractF := func(response ActionMessageResponse) error { return nil }
	err = r.client.ReceivesRpcActionOperate(msgs, corrId, extractF)
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
	// generate a temporary queue
	temporaryQueue, err := r.client.DeclareTemporaryQueueRpcActionOperate()
	if err != nil {
		return res, RpcError{msg: fmt.Errorf("GetAction: %w: %w", ErrCreateTemporaryQueue, err).Error(), errs: []error{ErrCreateTemporaryQueue, err}}
	}

	msgs, err := r.client.ConsumerTemporaryQueueRpcActionOperate(temporaryQueue)
	if err != nil {
		return res, RpcError{msg: fmt.Errorf("GetAction: %w: %w", ErrrCreateConsumer, err).Error(), errs: []error{ErrrCreateConsumer, err}}
	}

	rpc_queue, err := r.client.DeclareQueueRpcActionOperate()
	if err != nil {
		return res, RpcError{msg: fmt.Errorf("GetAction: %w: %w", ErrCreateActionOperateRPCQueue, err).Error(), errs: []error{ErrCreateActionOperateRPCQueue, err}}
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
	err = r.client.RequestRpcActionOperate(rpc_queue, temporaryQueue, corrId, reqByte)
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
	err = r.client.ReceivesRpcActionOperate(msgs, corrId, extractF)
	if err != nil {
		return res, fmt.Errorf("GetAction: %w", err)
	}
	res = GetActionResponse(resContent)
	return res, nil
}

type UpdateActionRequest struct {
	Selector SelectOne   `json:"selector"`
	Action   ActionModel `json:"action"`
}

func (r RPC) UpdateAction(req UpdateActionRequest) error {
	// generate a temporary queue
	temporaryQueue, err := r.client.DeclareTemporaryQueueRpcActionOperate()
	if err != nil {
		return RpcError{msg: fmt.Errorf("UpdateAction: %w: %w", ErrCreateTemporaryQueue, err).Error(), errs: []error{ErrCreateTemporaryQueue, err}}
	}

	msgs, err := r.client.ConsumerTemporaryQueueRpcActionOperate(temporaryQueue)
	if err != nil {
		return RpcError{msg: fmt.Errorf("UpdateAction: %w: %w", ErrrCreateConsumer, err).Error(), errs: []error{ErrrCreateConsumer, err}}
	}

	rpc_queue, err := r.client.DeclareQueueRpcActionOperate()
	if err != nil {
		return RpcError{msg: fmt.Errorf("UpdateAction: %w: %w", ErrCreateActionOperateRPCQueue, err).Error(), errs: []error{ErrCreateActionOperateRPCQueue, err}}
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
	err = r.client.RequestRpcActionOperate(rpc_queue, temporaryQueue, corrId, reqByte)
	if err != nil {
		return RpcError{msg: fmt.Errorf("UpdateAction: %s: %s", ErrPublishMessage, err).Error(), errs: []error{ErrPublishMessage, err}}
	}

	extractF := func(response ActionMessageResponse) error { return nil }
	err = r.client.ReceivesRpcActionOperate(msgs, corrId, extractF)
	if err != nil {
		return fmt.Errorf("UpdateAction: %w", err)
	}
	return nil
}

type DeleteActionRequest struct {
	Selector SelectOne `json:"selector"`
}

func (r RPC) DeleteAction(req DeleteActionRequest) error {
	// generate a temporary queue
	temporaryQueue, err := r.client.DeclareTemporaryQueueRpcActionOperate()
	if err != nil {
		return RpcError{msg: fmt.Errorf("DeleteAction: %w: %w", ErrCreateTemporaryQueue, err).Error(), errs: []error{ErrCreateTemporaryQueue, err}}
	}

	msgs, err := r.client.ConsumerTemporaryQueueRpcActionOperate(temporaryQueue)
	if err != nil {
		return RpcError{msg: fmt.Errorf("DeleteAction: %w: %w", ErrrCreateConsumer, err).Error(), errs: []error{ErrrCreateConsumer, err}}
	}

	rpc_queue, err := r.client.DeclareQueueRpcActionOperate()
	if err != nil {
		return RpcError{msg: fmt.Errorf("DeleteAction: %w: %w", ErrCreateActionOperateRPCQueue, err).Error(), errs: []error{ErrCreateActionOperateRPCQueue, err}}
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
	err = r.client.RequestRpcActionOperate(rpc_queue, temporaryQueue, corrId, reqByte)
	if err != nil {
		return RpcError{msg: fmt.Errorf("DeleteAction: %s: %s", ErrPublishMessage, err).Error(), errs: []error{ErrPublishMessage, err}}
	}

	extractF := func(response ActionMessageResponse) error { return nil }
	err = r.client.ReceivesRpcActionOperate(msgs, corrId, extractF)
	if err != nil {
		return fmt.Errorf("DeleteAction: %w", err)
	}
	return nil
}

type StopActionRequest struct {
	Selector SelectOne `json:"selector"`
}

func (r RPC) StopAction(req DeleteActionRequest) error {
	// generate a temporary queue
	temporaryQueue, err := r.client.DeclareTemporaryQueueRpcActionOperate()
	if err != nil {
		return RpcError{msg: fmt.Errorf("StopAction: %w: %w", ErrCreateTemporaryQueue, err).Error(), errs: []error{ErrCreateTemporaryQueue, err}}
	}

	msgs, err := r.client.ConsumerTemporaryQueueRpcActionOperate(temporaryQueue)
	if err != nil {
		return RpcError{msg: fmt.Errorf("StopAction: %w: %w", ErrrCreateConsumer, err).Error(), errs: []error{ErrrCreateConsumer, err}}
	}

	rpc_queue, err := r.client.DeclareQueueRpcActionOperate()
	if err != nil {
		return RpcError{msg: fmt.Errorf("StopAction: %w: %w", ErrCreateActionOperateRPCQueue, err).Error(), errs: []error{ErrCreateActionOperateRPCQueue, err}}
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
	err = r.client.RequestRpcActionOperate(rpc_queue, temporaryQueue, corrId, reqByte)
	if err != nil {
		return RpcError{msg: fmt.Errorf("StopAction: %s: %s", ErrPublishMessage, err).Error(), errs: []error{ErrPublishMessage, err}}
	}

	extractF := func(response ActionMessageResponse) error { return nil }
	err = r.client.ReceivesRpcActionOperate(msgs, corrId, extractF)
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
	errCh := make(chan error, 2)
	logCh := make(chan WatchActionLogResponse)
	go func() {
		// generate a temporary queue
		temporaryQueue, err := r.client.DeclareTemporaryQueueRpcActionOperate()
		if err != nil {
			errCh <- RpcError{msg: fmt.Errorf("UpdateAction: %w: %w", ErrCreateTemporaryQueue, err).Error(), errs: []error{ErrCreateTemporaryQueue, err}}
			return
		}

		msgs, err := r.client.ConsumerTemporaryQueueRpcActionOperate(temporaryQueue)
		if err != nil {
			errCh <- RpcError{msg: fmt.Errorf("UpdateAction: %w: %w", ErrrCreateConsumer, err).Error(), errs: []error{ErrrCreateConsumer, err}}
			return
		}

		rpc_queue, err := r.client.DeclareQueueRpcActionOperate()
		if err != nil {
			errCh <- RpcError{msg: fmt.Errorf("UpdateAction: %w: %w", ErrCreateActionOperateRPCQueue, err).Error(), errs: []error{ErrCreateActionOperateRPCQueue, err}}
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
			case d := <-msgs:
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
					started = true
					errCh <- nil

					res := WatchActionLogResContent{}
					err = json.Unmarshal([]byte(response.Content), &res)
					if err != nil {
						errCh <- err
						return
					}
					if res.WatchStatus == WatchStatusSending {
						logCh <- WatchActionLogResponse(res.Log)
					} else if res.WatchStatus == WatchStatusClose {
						close(logCh)
						return
					}
				}
			case <-time.After(expireTime):
				if !started {
					errCh <- RpcError{msg: fmt.Errorf("UpdateAction: %s and %s", ErrRetry, ErrRequestTimeout).Error(), errs: []error{ErrRetry, ErrRequestTimeout}}
					return
				}
			case <-ctx.Done():
				close(logCh)
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
