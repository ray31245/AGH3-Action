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
	"sort"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	actionv1 "github.com/Leukocyte-Lab/AGH3-Action/api/v1"
	corev1 "k8s.io/api/core/v1"
)

// ActionReconciler reconciles a Action object
type ActionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=action.lkc-lab.com,resources=actions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=action.lkc-lab.com,resources=actions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=action.lkc-lab.com,resources=actions/finalizers,verbs=update

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
		log.Error(err, "unable to fetch Action")
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
		for _, item := range childWorkers.Items[:len(childWorkers.Items)-1] {
			historyWorkers = append(historyWorkers, &item)
		}
	}

	isWorkerFinished := func(job *batchv1.Job) (bool, batchv1.JobConditionType) {
		for _, c := range job.Status.Conditions {
			if (c.Type == batchv1.JobComplete || c.Type == batchv1.JobFailed) && c.Status == corev1.ConditionTrue {
				return true, c.Type
			}
		}

		return false, ""
	}

	// update action's status
	action.Status.ActiveStatus = actionv1.ActiveStatusPending
	if lastWorker != nil {
		_, finishType := isWorkerFinished(lastWorker)
		switch finishType {
		case "":
			action.Status.ActiveStatus = actionv1.ActiveStatusRuning
		case batchv1.JobFailed:
			action.Status.ActiveStatus = actionv1.ActiveStatusFail
		case batchv1.JobComplete:
			action.Status.ActiveStatus = actionv1.ActiveStatusSuccessed
		}
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
		ttltime := int32(3600)
		worker := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				Name:        action.Name,
				Namespace:   action.Namespace,
			},
			Spec: batchv1.JobSpec{
				BackoffLimit:            &backoffLimit,
				TTLSecondsAfterFinished: &ttltime,
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

	// create it on the cluster
	if action.Spec.Activation && action.Status.ActiveStatus != actionv1.ActiveStatusRuning {
		if err = r.Create(ctx, worker); err != nil {
			log.Error(err, "unable to create Worker for Action", "Worker", worker)
			return ctrl.Result{}, err
		}
		action.Spec.Activation = false
		log.V(1).Info("create Worker for Action run", "worker", worker)
	}

	if err := r.Update(ctx, &action); err != nil {
		log.Error(err, "unable to update action status")
		return ctrl.Result{}, err
	}
	if err := r.Status().Update(ctx, &action); err != nil {
		log.Error(err, "unable to update Action status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

var (
	jobOwnerKey = ".metadata.controller"
	apiGVStr    = actionv1.GroupVersion.String()
)

// SetupWithManager sets up the controller with the Manager.
func (r *ActionReconciler) SetupWithManager(mgr ctrl.Manager) error {
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
	return ctrl.NewControllerManagedBy(mgr).
		For(&actionv1.Action{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
