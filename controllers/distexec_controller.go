/*
Copyright 2022.

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

package controllers

import (
	"bytes"
	"context"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	execv1 "github.com/gu18168/dist-exe/api/v1"
)

const (
	suffix        = "-daemonset"
	labelKey      = "dist-exec"
	containerName = "dist-exec-environment"
)

// DistExecReconciler reconciles a DistExec object
type DistExecReconciler struct {
	client.Client
	RESTClient rest.Interface
	RESTConfig *rest.Config
	Scheme     *runtime.Scheme
}

//+kubebuilder:rbac:groups=exec.yuhong.test,resources=distexecs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=exec.yuhong.test,resources=distexecs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=exec.yuhong.test,resources=distexecs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DistExec object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *DistExecReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var distExec execv1.DistExec
	if err := r.Get(ctx, req.NamespacedName, &distExec); err != nil {
		logger.Error(err, "DistExec name: resource not found", "name", req.NamespacedName)
		return ctrl.Result{}, err
	}

	var daemonSet *appsv1.DaemonSet
	err := r.Get(ctx, types.NamespacedName{
		Name:      distExec.Name + suffix,
		Namespace: distExec.Namespace,
	}, daemonSet)

	// If there is no corresponding DaemonSet, it means that it is a new custom resource.
	// We need to create the command execution environment.
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating new DaemonSet for name", req.NamespacedName)

		daemonSet = r.createDaemon(distExec.Name+suffix, distExec.Namespace)

		err = r.Create(ctx, daemonSet)
		if err != nil {
			logger.Error(err, "Fail to create DaemonSet")
			return ctrl.Result{}, err
		}
		// TODO: Is the Pod available immediately after the DaemonSet is created?
	} else if err != nil {
		logger.Error(err, "Fail to find DaemonSet")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// Use RESTClient to build a request and run a command in the Pod
func (r *DistExecReconciler) execInPod(name, namespace, command string) (string, string, error) {
	req := r.RESTClient.Post().
		Resource("pods").
		Name(name).
		Namespace(namespace).
		SubResource("exec").
		Param("container", containerName).
		VersionedParams(&corev1.PodExecOptions{
			Command: strings.Split(command, " "),
			Stdin:   false,
			Stdout:  true,
			Stderr:  true,
		}, runtime.NewParameterCodec(r.Scheme))

	var stdout, stderr bytes.Buffer
	exec, err := remotecommand.NewSPDYExecutor(r.RESTConfig, "POST", req.URL())
	if err != nil {
		return "", "", err
	}

	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return "", "", err
	}

	return stdout.String(), stderr.String(), nil
}

// DaemonSet ensures that there is a Pod on each Node to execute commands.
// So, we use DaemonSet to create the command execution environment.
func (r *DistExecReconciler) createDaemon(name, namespace string) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					labelKey: name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						labelKey: name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  containerName,
							Image: "busybox",
						},
					},
				},
			},
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *DistExecReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&execv1.DistExec{}).
		Complete(r)
}
