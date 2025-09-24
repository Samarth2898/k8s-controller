/*
Copyright 2025.

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
	"fmt"

	"github.com/slack-go/slack"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	crdv1 "sam.text/controller/api/v1"
)

// PodTrackerReconciler reconciles a PodTracker object
type PodTrackerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=crd.sam.test,resources=podtrackers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=crd.sam.test,resources=podtrackers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=crd.sam.test,resources=podtrackers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the PodTracker object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *PodTrackerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	var podTrackerList crdv1.PodTrackerList

	if err := r.List(ctx, &podTrackerList); err != nil {
		logger.Error(err, "unable to fetch pod tracker list")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if len(podTrackerList.Items) == 0 {
		logger.V(1).Info("no pod trackers configured")
		return ctrl.Result{}, nil
	} else {
		var podObject corev1.Pod
		err := r.Get(context.Background(), req.NamespacedName, &podObject)
		if err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		logger.V(1).Info("found repoter configured. sending report")
		report(podTrackerList.Items[0], podObject)
	}
	return ctrl.Result{}, nil
}

func report(reporter crdv1.PodTracker, pod corev1.Pod) {
	// report to slack
	logf.Log.V(1).Info("Reporting to reporter", "name", reporter.Spec.Name, "endpoint", reporter.Spec.Report.Key)
	slackChannel := reporter.Spec.Report.Channel
	app := slack.New(reporter.Spec.Report.Key, slack.OptionDebug(true))

	message := fmt.Sprintf("New pod created: %s", pod.Name)
	msgText := slack.NewTextBlockObject("mrkdwn", message, false, false)
	msgSection := slack.NewSectionBlock(msgText, nil, nil)
	msg := slack.MsgOptionBlocks(
		msgSection,
	)
	fmt.Print(msg)
	logf.Log.V(1).Info("Reporting", "message", "", "channel", slackChannel)
	_, _, _, err := app.SendMessage(slackChannel, msg)

	if err != nil {
		logf.Log.V(1).Info(err.Error())
	}
}
func (r *PodTrackerReconciler) HandlePodEvents(pod client.Object) []reconcile.Request {
	// TODO(user): your logic here
	if pod.GetNamespace() != "default" {
		return []reconcile.Request{}
	}

	namesapcedName := types.NamespacedName{
		Namespace: pod.GetNamespace(),
		Name:      pod.GetName(),
	}

	var podObject corev1.Pod
	err := r.Get(context.Background(), namesapcedName, &podObject)

	if err != nil {
		return []reconcile.Request{}
	}

	if len(podObject.Annotations) == 0 {
		logf.Log.V(1).Info("No annotations set, so this pod is becoming a tracked one now", "pod", podObject.Name)
	} else if podObject.GetAnnotations()["exampleAnnotation"] == "crd.devops.toolbox" {
		logf.Log.V(1).Info("Found a managed pod, lets report it", "pod", podObject.Name)
	} else {
		return []reconcile.Request{}
	}

	podObject.SetAnnotations(map[string]string{
		"exampleAnnotation": "crd.devops.toolbox",
	})

	if err := r.Update(context.TODO(), &podObject); err != nil {
		logf.Log.V(1).Info("error trying to update pod", "err", err)
	}
	requests := []reconcile.Request{
		{NamespacedName: namesapcedName},
	}
	return requests
	// return []reconcile.Request{}
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodTrackerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&crdv1.PodTracker{}).
		Named("podtracker").
		Complete(r)
}
