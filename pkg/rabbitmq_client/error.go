package rabbitmqclient

import (
	"errors"
	"fmt"
	"slices"
)

type ErrorCode uint32

const (
	ErrUnknow ErrorCode = iota
	ErrorCodeCreateTemporaryQueue
	ErrorCodeCreateActionOperateRPCQueue
	ErrorCodeCreateConsumer
	ErrorCodeUnmarshalRequest
	ErrorCodeUnmarshalRequestContent
	ErrorCodeCreateAction
	ErrorCodeActionAlreadyExist
	ErrorCodeActionNotfound
	ErrorCodeRetry
	ErrorCodeRequestTimeout
	ErrorCodeMarshalRequestContent
	ErrorCodeMarshalRequest
	ErrorCodePublishMessage
	ErrorCodeUnmarshalResponse
	ErrorCodeMarshalResponseContent
	ErrorCodeOperateNotSupport
)

var (
	ErrCreateTemporaryQueue        = errors.New("failed to declare a temporary queue")
	ErrCreateActionOperateRPCQueue = errors.New("failed to declare a rpc queue")
	ErrrCreateConsumer             = errors.New("failed to register a consumer")
	ErrUnmarshalRequest            = errors.New("unable to unmarshal request")
	ErrUnmarshalRequestContent     = errors.New("unable to unmarshal request content")
	ErrCreateAction                = errors.New("failed to create action")
	ErrActionAlreadyExist          = errors.New("action already exist")
	ErrActionNotfound              = errors.New("action notfound")
	ErrRetry                       = errors.New("retry later")
	ErrRequestTimeout              = errors.New("request timeout")
	ErrMarshalRequestContent       = errors.New("failed to marshal request content")
	ErrMarshalRequest              = errors.New("failed to marshal request")
	ErrPublishMessage              = errors.New("failed to publish a message")
	ErrUnmarshalResponse           = errors.New("failed to unmarshal response")
	ErrMarshalResponseContent      = errors.New("failed to marshal response content")
	ErrOperateNotSupport           = errors.New("operate not support")
)

type RpcError struct {
	msg  string
	errs []error
}

func (e RpcError) Error() string {
	return e.msg
}

func (e RpcError) Is(target error) bool {
	for _, v := range e.errs {
		if ok := errors.Is(v, target); ok {
			return true
		}
	}
	return false
}

func (e RpcError) Unwrap() []error {
	res := []error{}
	res = append(res, e.errs...)
	return res
}

// TODO: make a map for convert ErrorCodes to Err
func (msg ActionMessageResponse) extractError() (bool, RpcError) {
	if len(msg.ErrorCodes) > 0 || msg.ErrorMsg != "" {
		errs := []error{}
		if slices.Contains(msg.ErrorCodes, ErrorCodeActionAlreadyExist) {
			errs = append(errs, ErrActionAlreadyExist)
		}
		if slices.Contains(msg.ErrorCodes, ErrorCodeActionNotfound) {
			errs = append(errs, ErrActionNotfound)
		}
		return true, RpcError{msg: fmt.Sprintf("ActionMessageResponse.extractError: %s", msg.ErrorMsg), errs: errs}
	}
	return false, RpcError{}
}

func (msg ActionResultMessage) extractGetLogError() (bool, RpcError) {
	if len(msg.LogErrorCodes) > 0 || msg.LogErrorMsg != "" {
		errs := []error{}
		if slices.Contains(msg.LogErrorCodes, ErrorCodeActionAlreadyExist) {
			errs = append(errs, ErrActionAlreadyExist)
		}
		if slices.Contains(msg.LogErrorCodes, ErrorCodeActionNotfound) {
			errs = append(errs, ErrActionNotfound)
		}
		return true, RpcError{msg: fmt.Sprintf("ActionMessageResponse.extractError: %s", msg.LogErrorMsg), errs: errs}
	}
	return false, RpcError{}
}
