/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ref "k8s.io/client-go/tools/reference"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	actionv1 "github.com/Leukocyte-Lab/AGH3-Action/api/v1"
	HttpService "github.com/Leukocyte-Lab/AGH3-Action/pkg/http_service/action"
	mqService "github.com/Leukocyte-Lab/AGH3-Action/pkg/queue_service/action"
	corev1 "k8s.io/api/core/v1"
)

// ActionReconciler reconciles a Action object
type ActionReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	lifecycleHook lifecycleHook
}

//+kubebuilder:rbac:groups=action.lkc-lab.com,resources=actions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=action.lkc-lab.com,resources=actions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=action.lkc-lab.com,resources=actions/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Action object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *ActionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var action actionv1.Action
	if err := r.Get(ctx, req.NamespacedName, &action); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "unable to fetch Action")
		}
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var childWorkers batchv1.JobList
	var lastWorker *batchv1.Job
	var historyWorkers []*batchv1.Job
	if err := r.List(ctx, &childWorkers, client.InNamespace(req.Namespace), client.MatchingFields{jobOwnerKey: req.Name}); err != nil {
		log.Error(err, "unable to list child Workers")
		return ctrl.Result{}, err
	}

	sort.Slice(childWorkers.Items, func(i, j int) bool {
		if childWorkers.Items[i].Status.StartTime == nil {
			return childWorkers.Items[j].Status.StartTime != nil
		}
		return childWorkers.Items[i].Status.StartTime.Before(childWorkers.Items[j].Status.StartTime)
	})

	if len(childWorkers.Items) > 0 {
		lastWorker = &childWorkers.Items[len(childWorkers.Items)-1]
		for i := range childWorkers.Items[:len(childWorkers.Items)-1] {
			// if StartTime is nil, sort is wrong
			if childWorkers.Items[i].Status.StartTime == nil {
				return ctrl.Result{}, nil
			}
			historyWorkers = append(historyWorkers, &childWorkers.Items[i])
		}
	}

	isWorkerFinished := func(job *batchv1.Job) (bool, batchv1.JobConditionType) {
		for _, c := range job.Status.Conditions {
			if (c.Type == batchv1.JobComplete || c.Type == batchv1.JobFailed || c.Type == batchv1.JobSuspended) && c.Status == corev1.ConditionTrue {
				return true, c.Type
			}
		}

		return false, ""
	}

	// update action's status
	targetActiveStatus := actionv1.ActiveStatusPending
	if action.Status.ActiveStatus != "" {
		targetActiveStatus = action.Status.ActiveStatus
	}
	WorkerRef := (*corev1.ObjectReference)(nil)
	if lastWorker != nil {
		if worker, err := ref.GetReference(r.Scheme, lastWorker); err != nil {
			log.Error(err, "unable to make reference to worker")
		} else {
			WorkerRef = worker
		}

		_, finishType := isWorkerFinished(lastWorker)
		switch finishType {
		case "":
			targetActiveStatus = actionv1.ActiveStatusRuning
		case batchv1.JobFailed:
			targetActiveStatus = actionv1.ActiveStatusFail
		case batchv1.JobComplete:
			targetActiveStatus = actionv1.ActiveStatusSuccessed
		case batchv1.JobSuspended:
			targetActiveStatus = actionv1.ActiveStatusStop
		}
	}

	// sync targetActiveStatus and action status
	if targetActiveStatus != action.Status.ActiveStatus {
		err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			if err := r.Get(ctx, types.NamespacedName{Name: action.Name, Namespace: action.Namespace}, &action); err != nil {
				return err
			}
			action.Status.ActiveStatus = targetActiveStatus
			action.Status.Worker = WorkerRef
			return r.Status().Update(ctx, &action)
		})
		if err != nil {
			log.Error(err, "unable to update Action status")
			return ctrl.Result{}, err
		}
		if targetActiveStatus == actionv1.ActiveStatusSuccessed || targetActiveStatus == actionv1.ActiveStatusFail {
			if r.lifecycleHook.ActionFinish != nil {
				for _, f := range r.lifecycleHook.ActionFinish {
					f(action)
				}
			}
			if targetActiveStatus == actionv1.ActiveStatusSuccessed {
				if r.lifecycleHook.ActionSuccess != nil {
					for _, f := range r.lifecycleHook.ActionSuccess {
						f(action)
					}
				}
			}
		}
		return ctrl.Result{}, nil
	}

	// clean up old worker
	for i, worker := range historyWorkers {
		if int32(i) >= int32(len(historyWorkers))-*action.Spec.WorkerHistoryLimit {
			break
		}
		if err := r.Delete(ctx, worker, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
			log.Error(err, "unable to delete old worker", "worker", worker)
		} else {
			log.V(0).Info("delete old worker", "worker", worker)
		}
	}

	contructWorkerForAction := func(action *actionv1.Action) (*batchv1.Job, error) {
		backoffLimit := int32(0)
		ttlTime := int32(3600)
		actimeTime := int64(10800)
		worker := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				Name:        fmt.Sprintf("%s-%d", action.Name, time.Now().Unix()),
				Namespace:   action.Namespace,
			},
			Spec: batchv1.JobSpec{
				BackoffLimit:            &backoffLimit,
				TTLSecondsAfterFinished: &ttlTime,
				ActiveDeadlineSeconds:   &actimeTime,
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Name: action.Name,
						Labels: map[string]string{
							"podName": action.Name,
							"kind":    "worker",
						},
					},
					Spec: corev1.PodSpec{
						NodeSelector: map[string]string{
							"purpose": "worker",
						},
						RestartPolicy: corev1.RestartPolicyNever,
						Containers: []corev1.Container{
							{
								Name:            "main",
								Image:           action.Spec.Image,
								Args:            action.Spec.Args,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("500m"),
										corev1.ResourceMemory: resource.MustParse("512Mi"),
									},
								},
							},
						},
					},
				},
			},
		}
		if err := ctrl.SetControllerReference(action, worker, r.Scheme); err != nil {
			return nil, err
		}

		return worker, nil
	}

	worker, err := contructWorkerForAction(&action)
	if err != nil {
		log.Error(err, "unable to contruct worker from template")
		return ctrl.Result{}, err
	}

	// stop action's worker
	if action.Spec.TrigerStop {
		if lastWorker != nil {
			err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
				if err := r.Get(ctx, types.NamespacedName{Name: lastWorker.Name, Namespace: lastWorker.Namespace}, lastWorker); err != nil {
					return err
				}
				suspend := true
				lastWorker.Spec.Suspend = &suspend
				return r.Update(ctx, lastWorker)
			})
			if err != nil {
				log.Error(err, "unable to update lastWorker")
				return ctrl.Result{}, err
			}
		}
		action.Spec.TrigerStop = false
	}

	// create it on the cluster
	if action.Spec.TrigerRun {
		if action.Status.ActiveStatus != actionv1.ActiveStatusRuning {
			if err = r.Create(ctx, worker); err != nil {
				log.Error(err, "unable to create Worker for Action", "Worker", worker)
				return ctrl.Result{}, err
			}
			log.V(1).Info("create Worker for Action run", "worker", worker)
		} else {
			log.V(1).Info("unable to activation action, due to action is running", "action", action)
		}
		action.Spec.TrigerRun = false
	}

	if err := r.Update(ctx, &action); err != nil && !apierrors.IsConflict(err) {
		log.Error(err, "unable to update action")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ActionReconciler) CreateAction(action *actionv1.Action) error {
	if err := r.Create(context.Background(), action); err != nil {
		return fmt.Errorf("ActionReconciler.CreateAction: %w", err)
	}
	return nil
}

func (r *ActionReconciler) GetAction(name string, nameSpace string) (*actionv1.Action, error) {
	var action actionv1.Action
	err := r.Get(context.Background(), types.NamespacedName{Name: name, Namespace: nameSpace}, &action)
	if err != nil {
		return nil, fmt.Errorf("ActionReconciler.GetAction: %w", err)
	}
	return &action, nil
}

func (r *ActionReconciler) GetPodByAction(action *actionv1.Action) (*corev1.Pod, error) {
	if action.Status.Worker == nil {
		return nil, errors.New("this action does not have worker")
	}
	worker := action.Status.Worker
	childPods := corev1.PodList{}
	if err := r.List(context.Background(), &childPods, client.InNamespace(worker.Namespace), client.MatchingFields{jobOwnerKey: worker.Name}); err != nil {
		return nil, fmt.Errorf("failed to get worker's pod: %w", err)
	}
	if len(childPods.Items) == 0 {
		return nil, errors.New("this action's worker is not running")
	}
	return &childPods.Items[0], nil
}

func (r *ActionReconciler) DeleteAction(action *actionv1.Action) error {
	if err := r.Delete(context.Background(), action, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("ActionReconciler.DeleteAction: %w", err)
	}
	return nil
}

func (r *ActionReconciler) UpdateAction(action *actionv1.Action) error {
	if err := r.Update(context.Background(), action); err != nil {
		return fmt.Errorf("ActionReconciler.UpdateAction: %w", err)
	}
	return nil
}
func (r *ActionReconciler) GetRunningActionCount() (int, error) {
	// var ownPod corev1.PodList
	var runningActions actionv1.ActionList
	if err := r.List(context.Background(), &runningActions, client.MatchingFields{activeStatusKey: string(actionv1.ActiveStatusRuning)}); err != nil {
		return 0, fmt.Errorf("ActionReconciler.GetRunningAction: %w", err)
	}
	return len(runningActions.Items), nil
}

func (r *ActionReconciler) GetActionsByHistoryID(historyID string) ([]actionv1.Action, error) {
	var actionList actionv1.ActionList
	if err := r.List(context.Background(), &actionList, client.MatchingFields{historyIDKey: historyID}); err != nil {
		return nil, fmt.Errorf("ActionReconciler.GetActionsByHistoryID: %w", err)
	}
	return actionList.Items, nil
}

type lifecycleHook struct {
	ActionFinish  []func(actionv1.Action)
	ActionSuccess []func(actionv1.Action)
}

type HookRegister struct {
	hook *lifecycleHook
}

func (h *HookRegister) OnSuccess(f func(actionv1.Action)) {
	h.hook.ActionSuccess = append(h.hook.ActionSuccess, f)
}

func (h *HookRegister) OnFinish(f func(actionv1.Action)) {
	h.hook.ActionFinish = append(h.hook.ActionFinish, f)
}

var (
	jobOwnerKey     = ".metadata.controller"
	activeStatusKey = "status.activeStatus"
	historyIDKey    = "spec.historyID"
	apiGVStr        = actionv1.GroupVersion.String()
)

// SetupWithManager sets up the controller with the Manager.
func (r *ActionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	config := mgr.GetConfig()
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("cannot new k8s client: %w", err)
	}
	queueService, err := mqService.New(r, mgr.GetLogger(), clientSet)
	if err != nil {
		return fmt.Errorf("fail to setup Rabbitmq: %w", err)
	}
	if err = queueService.Run(&HookRegister{&r.lifecycleHook}); err != nil {
		return fmt.Errorf("fail to run Rabbitmq: %w", err)
	}

	WebhookService := HttpService.New(r)
	go func() {
		WebhookService.Run()
	}()
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &batchv1.Job{}, jobOwnerKey, func(rawObj client.Object) []string {
		// grab the job object, extract the owner...
		job := rawObj.(*batchv1.Job)
		owner := metav1.GetControllerOf(job)
		if owner == nil {
			return nil
		}
		// ...make sure it's a CronJob...
		if owner.APIVersion != apiGVStr || owner.Kind != "Action" {
			return nil
		}

		// ...and if so, return it
		return []string{owner.Name}
	}); err != nil {
		return err
	}
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.Pod{}, jobOwnerKey, func(rawObj client.Object) []string {
		// grab the job object, extract the owner...
		pod := rawObj.(*corev1.Pod)
		owner := metav1.GetControllerOf(pod)
		if owner == nil {
			return nil
		}
		// ...make sure it's a CronJob...
		if owner.APIVersion != batchv1.SchemeGroupVersion.String() || owner.Kind != "Job" {
			return nil
		}

		// ...and if so, return it
		return []string{owner.Name}
	}); err != nil {
		return err
	}
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &actionv1.Action{}, activeStatusKey, func(rawObj client.Object) []string {
		action := rawObj.(*actionv1.Action)
		return []string{string(action.Status.ActiveStatus)}
	}); err != nil {
		return err
	}
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &actionv1.Action{}, historyIDKey, func(rawObj client.Object) []string {
		action := rawObj.(*actionv1.Action)
		return []string{string(action.Spec.HistoryID)}
	}); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&actionv1.Action{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
