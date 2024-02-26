package action

import (
	// "io"
	// "net/http"
	// "strconv"
	// "strings"

	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	actionv1 "github.com/Leukocyte-Lab/AGH3-Action/api/v1"
	rabbitmqClient "github.com/Leukocyte-Lab/AGH3-Action/pkg/rabbitmq_client"
	"github.com/go-logr/logr"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	// "sigs.k8s.io/controller-runtime/pkg/client"
	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	workerHistoryLimit = int32(3)
)

type ActionControllerInterface interface {
	CreateAction(*actionv1.Action) error
	GetAction(name string, nameSpace string) (*actionv1.Action, error)
	DeleteAction(*actionv1.Action) error
	UpdateAction(action *actionv1.Action) error
}

type RabbitmqService struct {
	ac           ActionControllerInterface
	client       rabbitmqClient.RabbitmqClient
	logger       logr.Logger
	k8sInterface kubernetes.Interface
}

func New(ac ActionControllerInterface, logger logr.Logger, clientSet kubernetes.Interface) (RabbitmqService, error) {
	client, err := rabbitmqClient.New("amqp://guest:guest@localhost:5672/")
	if err != nil {
		return RabbitmqService{}, fmt.Errorf("failed to New rabbitClient: %w", err)
	}

	err = client.Channel.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		return RabbitmqService{}, fmt.Errorf("failed to set Qos")
	}

	res := RabbitmqService{
		ac:           ac,
		client:       client,
		logger:       logger,
		k8sInterface: clientSet,
	}

	return res, nil
}

func (service RabbitmqService) Run() error {
	client := service.client
	q, err := client.DeclareQueueRpcActionOperate()
	if err != nil {
		return fmt.Errorf("fail to declare a DeclareQueueRpcActionOperate: %w", err)
	}

	operateMessges, err := client.Channel.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    //args
	)
	if err != nil {
		return fmt.Errorf("fail to register a consumer: %w", err)
	}

	go func() {
		for d := range operateMessges {
			go func(d amqp.Delivery) {
				actMsg := rabbitmqClient.ActionOperateMessageRequest{}
				err := json.Unmarshal(d.Body, &actMsg)
				if err != nil {
					client.ResponseErrorMessage(d,
						fmt.Errorf("RabbitmqService.Run(): %w: %s", rabbitmqClient.ErrUnmarshalRequest, err).Error(),
						rabbitmqClient.ErrorCodeUnmarshalRequest,
					)
					return
				}
				switch actMsg.Operate {
				case rabbitmqClient.OperateCreate:
					service.createActionHandler(actMsg.Content, d)
				case rabbitmqClient.OperateGet:
					service.getActionHandler(actMsg.Content, d)
				case rabbitmqClient.OperateUpdate:
					service.updateActionHandler(actMsg.Content, d)
				case rabbitmqClient.OperateDelete:
					service.deleteActionHandler(actMsg.Content, d)
				case rabbitmqClient.OperateStop:
					service.stopActionHandler(actMsg.Content, d)
				case rabbitmqClient.OperateWatchLog:
					service.watchActionLogHandler(actMsg.Content, d)
				default:
					service.client.ResponseErrorMessage(d,
						fmt.Errorf("RabbitmqService: %w", rabbitmqClient.ErrOperateNotSupport).Error(),
						rabbitmqClient.ErrorCodeOperateNotSupport,
					)
				}
			}(d)
		}
	}()

	return nil
}

func (service RabbitmqService) createActionHandler(contentStr string, d amqp.Delivery) {
	content := rabbitmqClient.CreateActionReqContent{}
	err := json.Unmarshal([]byte(contentStr), &content)
	if err != nil {
		service.client.ResponseErrorMessage(d,
			fmt.Errorf("RabbitmqService.createActionHandler: %w: %w", rabbitmqClient.ErrUnmarshalRequestContent, err).Error(),
			rabbitmqClient.ErrorCodeUnmarshalRequestContent,
		)
		return
	}
	if content.Action.NameSpace == "" {
		content.Action.NameSpace = "default"
	}
	action := actionv1.Action{
		ObjectMeta: metav1.ObjectMeta{
			Name:      content.Action.Name,
			Namespace: content.Action.NameSpace,
		},
		Spec: actionv1.ActionSpec{
			Image:              content.Action.Image,
			Args:               content.Action.Args,
			WorkerHistoryLimit: &workerHistoryLimit,
		},
	}
	if err := service.ac.CreateAction(&action); err != nil {
		codes := []rabbitmqClient.ErrorCode{rabbitmqClient.ErrorCodeCreateAction}
		if exist := apierrors.IsAlreadyExists(err); exist {
			codes = append(codes, rabbitmqClient.ErrorCodeActionAlreadyExist)
		}

		service.client.ResponseErrorMessage(d,
			fmt.Errorf("RabbitmqService.createActionHandler: %w: %w", rabbitmqClient.ErrCreateAction, err).Error(),
			codes...,
		)
		return
	}
	service.client.ResponseMessage(d, "")
}

func (service RabbitmqService) getActionHandler(contentStr string, d amqp.Delivery) {
	content := rabbitmqClient.GetActionReqContent{}
	err := json.Unmarshal([]byte(contentStr), &content)
	if err != nil {
		service.client.ResponseErrorMessage(d,
			fmt.Errorf("RabbitmqService.getActionHandler: %w: %w", rabbitmqClient.ErrUnmarshalRequestContent, err).Error(),
			rabbitmqClient.ErrorCodeUnmarshalRequestContent,
		)
		return
	}
	if content.Selector.NameSpace == "" {
		content.Selector.NameSpace = "default"
	}
	action, err := service.ac.GetAction(content.Selector.Name, content.Selector.NameSpace)
	if err != nil {
		codes := []rabbitmqClient.ErrorCode{}
		if notfound := apierrors.IsNotFound(err); notfound {
			codes = append(codes, rabbitmqClient.ErrorCodeActionNotfound)
		}
		service.client.ResponseErrorMessage(d,
			fmt.Errorf("RabbitmqService.getActionHandler: %w", err).Error(),
			codes...,
		)
		return
	}
	resContent := rabbitmqClient.GetActionResContent{
		Action: rabbitmqClient.ActionModel{
			NameSpace: action.Namespace,
			Name:      action.Name,
			Image:     action.Spec.Image,
			Args:      action.Spec.Args,
		},
		Status: rabbitmqClient.ActionStatus(action.Status.ActiveStatus),
	}
	res, err := json.Marshal(resContent)
	if err != nil {
		service.client.ResponseErrorMessage(d,
			fmt.Errorf("RabbitmqService.getActionHandler: %w: %w", rabbitmqClient.ErrMarshalResponseContent, err).Error(),
		)
		return
	}
	service.client.ResponseMessage(d, string(res))
}

func (service RabbitmqService) updateActionHandler(contentStr string, d amqp.Delivery) {
	content := rabbitmqClient.UpdateActionReqContent{}
	err := json.Unmarshal([]byte(contentStr), &content)
	if err != nil {
		service.client.ResponseErrorMessage(d,
			fmt.Errorf("RabbitmqService.updateActionHandler: %w: %w", rabbitmqClient.ErrUnmarshalRequestContent, err).Error(),
			rabbitmqClient.ErrorCodeUnmarshalRequestContent,
		)
		return
	}
	if content.Selector.NameSpace == "" {
		content.Selector.NameSpace = "default"
	}
	if content.Action.NameSpace == "" {
		content.Action.NameSpace = "default"
	}
	// check new action exist or not, if exist return error
	_, err = service.ac.GetAction(content.Action.Name, content.Action.NameSpace)
	if err == nil {
		service.client.ResponseErrorMessage(d,
			fmt.Errorf("RabbitmqService.updateActionHandler: %w", rabbitmqClient.ErrActionAlreadyExist).Error(),
			rabbitmqClient.ErrorCodeActionAlreadyExist,
		)
		return
	} else if notfound := apierrors.IsNotFound(err); !notfound {
		service.client.ResponseErrorMessage(d,
			fmt.Errorf("RabbitmqService.updateActionHandler: %w", err).Error(),
		)
		return
	}

	oldAction, err := service.ac.GetAction(content.Selector.Name, content.Selector.NameSpace)
	if err != nil {
		codes := []rabbitmqClient.ErrorCode{}
		if notfound := apierrors.IsNotFound(err); notfound {
			codes = append(codes, rabbitmqClient.ErrorCodeActionNotfound)
		}
		service.client.ResponseErrorMessage(d,
			fmt.Errorf("RabbitmqService.updateActionHandler: %w", err).Error(),
			codes...,
		)
		return
	}
	if err := service.ac.DeleteAction(oldAction); err != nil {
		codes := []rabbitmqClient.ErrorCode{}
		if notfound := apierrors.IsNotFound(err); notfound {
			codes = append(codes, rabbitmqClient.ErrorCodeActionNotfound)
		}
		service.client.ResponseErrorMessage(d,
			fmt.Errorf("RabbitmqService.updateActionHandler: %w", err).Error(),
			codes...,
		)
		return
	}
	newAction := &actionv1.Action{
		ObjectMeta: metav1.ObjectMeta{
			Name:      content.Action.Name,
			Namespace: content.Action.NameSpace,
		},
		Spec: actionv1.ActionSpec{
			Image:              content.Action.Image,
			Args:               content.Action.Args,
			WorkerHistoryLimit: &workerHistoryLimit,
		},
	}
	if err := service.ac.CreateAction(newAction); err != nil {
		codes := []rabbitmqClient.ErrorCode{rabbitmqClient.ErrorCodeCreateAction}
		if exist := apierrors.IsAlreadyExists(err); exist {
			codes = append(codes, rabbitmqClient.ErrorCodeActionAlreadyExist)
		}

		service.client.ResponseErrorMessage(d,
			fmt.Errorf("RabbitmqService.updateActionHandler: %w: %w", rabbitmqClient.ErrCreateAction, err).Error(),
			codes...,
		)
		return
	}
	service.client.ResponseMessage(d, "")
}

func (service RabbitmqService) deleteActionHandler(contentStr string, d amqp.Delivery) {
	content := rabbitmqClient.DeleteActionReqContent{}
	err := json.Unmarshal([]byte(contentStr), &content)
	if err != nil {
		service.client.ResponseErrorMessage(d,
			fmt.Errorf("RabbitmqService.deleteActionHandler: %w: %w", rabbitmqClient.ErrUnmarshalRequestContent, err).Error(),
			rabbitmqClient.ErrorCodeUnmarshalRequestContent,
		)
		return
	}
	if content.Selector.NameSpace == "" {
		content.Selector.NameSpace = "default"
	}

	action, err := service.ac.GetAction(content.Selector.Name, content.Selector.NameSpace)
	if err != nil {
		codes := []rabbitmqClient.ErrorCode{}
		if notfound := apierrors.IsNotFound(err); notfound {
			codes = append(codes, rabbitmqClient.ErrorCodeActionNotfound)
		}
		service.client.ResponseErrorMessage(d,
			fmt.Errorf("RabbitmqService.deleteActionHandler: %w", err).Error(),
			codes...,
		)
		return
	}
	if err := service.ac.DeleteAction(action); err != nil {
		codes := []rabbitmqClient.ErrorCode{}
		if notfound := apierrors.IsNotFound(err); notfound {
			codes = append(codes, rabbitmqClient.ErrorCodeActionNotfound)
		}
		service.client.ResponseErrorMessage(d,
			fmt.Errorf("RabbitmqService.deleteActionHandler: %w", err).Error(),
			codes...,
		)
		return
	}
	service.client.ResponseMessage(d, "")
}

func (service RabbitmqService) stopActionHandler(contentStr string, d amqp.Delivery) {
	content := rabbitmqClient.StopActionReqContent{}
	err := json.Unmarshal([]byte(contentStr), &content)
	if err != nil {
		service.client.ResponseErrorMessage(d,
			fmt.Errorf("RabbitmqService.stopActionHandler: %w: %w", rabbitmqClient.ErrUnmarshalRequestContent, err).Error(),
			rabbitmqClient.ErrorCodeUnmarshalRequestContent,
		)
		return
	}
	if content.Selector.NameSpace == "" {
		content.Selector.NameSpace = "default"
	}

	action, err := service.ac.GetAction(content.Selector.Name, content.Selector.NameSpace)
	if err != nil {
		codes := []rabbitmqClient.ErrorCode{}
		if notfound := apierrors.IsNotFound(err); notfound {
			codes = append(codes, rabbitmqClient.ErrorCodeActionNotfound)
		}
		service.client.ResponseErrorMessage(d,
			fmt.Errorf("RabbitmqService.stopActionHandler: %w", err).Error(),
			codes...,
		)
		return
	}
	action.Spec.TrigerStop = true
	if err := service.ac.UpdateAction(action); err != nil {
		codes := []rabbitmqClient.ErrorCode{}
		if notfound := apierrors.IsNotFound(err); notfound {
			codes = append(codes, rabbitmqClient.ErrorCodeActionNotfound)
		}
		service.client.ResponseErrorMessage(d,
			fmt.Errorf("RabbitmqService.stopActionHandler: %w", err).Error(),
			codes...,
		)
		return
	}
	service.client.ResponseMessage(d, "")
}

func (service RabbitmqService) watchActionLogHandler(contentStr string, d amqp.Delivery) {
	content := rabbitmqClient.WatchActionLogReqContent{}
	err := json.Unmarshal([]byte(contentStr), &content)
	if err != nil {
		service.client.ResponseErrorMessage(d,
			fmt.Errorf("RabbitmqService.getActionHandler: %w: %w", rabbitmqClient.ErrUnmarshalRequestContent, err).Error(),
			rabbitmqClient.ErrorCodeUnmarshalRequestContent,
		)
		return
	}
	if content.Selector.NameSpace == "" {
		content.Selector.NameSpace = "default"
	}
	action, err := service.ac.GetAction(content.Selector.Name, content.Selector.NameSpace)
	if err != nil {
		codes := []rabbitmqClient.ErrorCode{}
		if notfound := apierrors.IsNotFound(err); notfound {
			codes = append(codes, rabbitmqClient.ErrorCodeActionNotfound)
		}
		service.client.ResponseErrorMessage(d,
			fmt.Errorf("RabbitmqService.getActionHandler: %w", err).Error(),
			codes...,
		)
		return
	}
	// TODO: if needed scale controller and cancel this watch,this ctx should be contextWithCancel use a queue to receive signal
	watchCtx := context.Background()
	stream, err := service.k8sInterface.CoreV1().Pods(action.Namespace).GetLogs(action.Name, &v1.PodLogOptions{
		Container: "main",
		Follow:    true,
	}).Stream(watchCtx)
	if err != nil {
		service.client.ResponseErrorMessage(d,
			fmt.Errorf("RabbitmqService.getActionHandler: %w", err).Error(),
		)
		return
	}
	if stream == nil {
		service.client.ResponseErrorMessage(d,
			fmt.Errorf("RabbitmqService.getActionHandler: %w", errors.New("get nil stream")).Error(),
		)
		return
	}
	defer stream.Close()

	err = service.client.WatchActionLogResponse(d, rabbitmqClient.WatchStatusStart, "")
	if err != nil {
		service.client.ResponseErrorMessage(d,
			fmt.Errorf("RabbitmqService.getActionHandler: %w: %w", rabbitmqClient.ErrMarshalResponseContent, err).Error(),
		)
		return
	}

	buf := *bufio.NewReader(stream)
	var bufRrr error
	msg := ""
	for {
		msg, bufRrr = buf.ReadString(byte('\n'))
		if bufRrr != nil && bufRrr != io.EOF {
			service.logger.V(1).Info(fmt.Sprintf("RabbitmqService.getActionHandler: GetLogs.Stream read into buffer: %s", bufRrr.Error()))
			continue
		}

		// Send Action log
		err = service.client.WatchActionLogResponse(d, rabbitmqClient.WatchStatusSending, msg)
		if err != nil {
			service.client.ResponseErrorMessage(d,
				fmt.Errorf("RabbitmqService.getActionHandler: %w: %w", rabbitmqClient.ErrMarshalResponseContent, err).Error(),
			)
		}

		// Close Log stream
		if bufRrr == io.EOF {
			err = service.client.WatchActionLogResponse(d, rabbitmqClient.WatchStatusClose, "")
			if err != nil {
				service.client.ResponseErrorMessage(d,
					fmt.Errorf("RabbitmqService.getActionHandler: %w: %w", rabbitmqClient.ErrMarshalResponseContent, err).Error(),
				)
			}
			break
		}
	}
}
