package action

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	actionv1 "github.com/Leukocyte-Lab/AGH3-Action/api/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ActionSpec struct{}
type ActionControllerInterface interface {
	CreateAction(*actionv1.Action) error
	GetAction(name string, nameSpace string) (*actionv1.Action, error)
	DeleteAction(*actionv1.Action) error
	UpdateAction(action *actionv1.Action) error
}

type WebhookHttpService struct {
	ActionControllerInterface
	server *http.ServeMux
}

func New(ac ActionControllerInterface) WebhookHttpService {
	mu := http.NewServeMux()
	res := WebhookHttpService{
		ActionControllerInterface: ac,
		server:                    mu,
	}
	mu.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "sucessful access action controller") })
	mu.HandleFunc("/create", createActionHandler(ac))
	mu.HandleFunc("/delete", deleteActionHandler(ac))
	mu.HandleFunc("/run", RunActionHandler(ac))
	mu.HandleFunc("/stop", StopActionHandler(ac))
	mu.HandleFunc("/stressTest", StressTestHandler(ac))
	return res
}

func createActionHandler(controller ActionControllerInterface) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if name == "" {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, "fail to create action, need a name")
			return
		}
		nameSpace := r.URL.Query().Get("nameSpace")
		if nameSpace == "" {
			nameSpace = "default"
		}
		image := r.URL.Query().Get("image")
		if image == "" {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, "fail to create action, need a image")
			return
		}
		args := r.URL.Query().Get("args")
		argsSlice := strings.Split(args, ",")
		workerHistoryLimit := int32(3)
		action := actionv1.Action{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: nameSpace,
			},
			Spec: actionv1.ActionSpec{
				Image:              image,
				Args:               argsSlice,
				WorkerHistoryLimit: &workerHistoryLimit,
			},
		}
		if err := controller.CreateAction(&action); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, "fail to create action,"+err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "success to create action")
	}
}

func deleteActionHandler(controller ActionControllerInterface) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if name == "" {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, "fail to delete action, need a name")
			return
		}
		nameSpace := r.URL.Query().Get("nameSpace")
		if nameSpace == "" {
			nameSpace = "default"
		}
		action, err := controller.GetAction(name, nameSpace)
		if apierrors.IsNotFound(err) {
			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, "fail to delete action, not found")
			return
		} else if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, "fail to delete action"+err.Error())
			return
		}
		if err := controller.DeleteAction(action); client.IgnoreNotFound(err) != nil {
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, "fail to delete action"+err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "sucess delete action")
	}
}

func RunActionHandler(controller ActionControllerInterface) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if name == "" {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, "fail to run action, need a name")
			return
		}
		nameSpace := r.URL.Query().Get("nameSpace")
		if nameSpace == "" {
			nameSpace = "default"
		}
		action, err := controller.GetAction(name, nameSpace)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, "fail to run action"+err.Error())
			return
		}
		action.Spec.Activation = true
		if err := controller.UpdateAction(action); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, "fail to run action"+err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "success run action")
	}
}

func StopActionHandler(controller ActionControllerInterface) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if name == "" {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, "fail to stop action, need a name")
			return
		}
		nameSpace := r.URL.Query().Get("nameSpace")
		if nameSpace == "" {
			nameSpace = "default"
		}
		action, err := controller.GetAction(name, nameSpace)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, "fail to stop action"+err.Error())
			return
		}
		action.Spec.Stop = true
		if err := controller.UpdateAction(action); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, "fail to stop action"+err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "success stop action")
	}
}

func StressTestHandler(controller ActionControllerInterface) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		nameSpace := r.URL.Query().Get("nameSpace")
		if nameSpace == "" {
			nameSpace = "default"
		}
		image := r.URL.Query().Get("image")
		if image == "" {
			io.WriteString(w, "fail to create action, need a image")
			return
		}
		args := r.URL.Query().Get("args")
		argsSlice := strings.Split(args, ",")
		count := getParamIntDefault(r, "count", 0)
		workerHistoryLimit := int32(0)
		if count == 0 {
			io.WriteString(w, "unable to stress test with count 0")
			return
		}
		startTime := time.Now()
		for i := 1; i <= count; i++ {
			action := actionv1.Action{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "stresstest-" + strconv.Itoa(i),
					Namespace: nameSpace,
					Labels:    map[string]string{"stressTest": "true"},
				},
				Spec: actionv1.ActionSpec{
					Image:              image,
					Args:               argsSlice,
					WorkerHistoryLimit: &workerHistoryLimit,
				},
			}
			err := controller.CreateAction(&action)
			if err != nil {
				io.WriteString(w, "one of stress test unable to create:"+err.Error())
				return
			}
		}
		for i := 1; i <= count; i++ {
			action, err := controller.GetAction("stresstest-"+strconv.Itoa(i), nameSpace)
			if err != nil {
				io.WriteString(w, "one of stress action not found, can't run")
				return
			}
			action.Spec.Activation = true
			controller.UpdateAction(action)
		}
		timeoutCheck := time.Now()
		for i := 1; i <= count; i++ {
			if time.Now().After(timeoutCheck.Add(time.Hour * 2)) {
				io.WriteString(w, "stress test time out")
				return
			}
			action, err := controller.GetAction("stresstest-"+strconv.Itoa(i), nameSpace)
			if err != nil || action.Status.ActiveStatus == actionv1.ActiveStatusPending || action.Status.ActiveStatus == actionv1.ActiveStatusRuning {
				i = 1
				continue
			}
			if action.Status.ActiveStatus == actionv1.ActiveStatusFail {
				io.WriteString(w, "one of actions is fail, stress fail")
				return
			}
		}
		spendTime := time.Since(startTime)

		io.WriteString(w, "stress test success,spend "+spendTime.String())
	}
}

func getParamIntDefault(r *http.Request, name string, def int) int {
	q := r.URL.Query().Get(name)
	res, err := strconv.Atoi(q)
	if err != nil {
		return def
	}
	return res
}

func getParamBoolDefault(r *http.Request, name string, def bool) bool {
	q := r.URL.Query().Get(name)
	res, err := strconv.ParseBool(q)
	if err != nil {
		return def
	}
	return res
}

func (s WebhookHttpService) Run() {
	http.ListenAndServe(":8087", s.server)
}
