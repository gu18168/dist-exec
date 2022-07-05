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

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	execv1 "github.com/gu18168/dist-exe/api/v1"
)

const (
	podNamespace = "dist-exec"
	podLabel     = "dist-exec-pod"
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
	_ = log.FromContext(ctx)

	// TODO(user): your logic here

	return ctrl.Result{}, nil
}

// Use RESTClient to build a request and run a command in the Pod
func (r *DistExecReconciler) execInPod(name, command string) (string, string, error) {
	req := r.RESTClient.Post().
		Resource("pods").
		Name(name).
		Namespace(podNamespace).
		SubResource("exec").
		Param("container", podLabel).
		VersionedParams(&v1.PodExecOptions{
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

// SetupWithManager sets up the controller with the Manager.
func (r *DistExecReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&execv1.DistExec{}).
		Complete(r)
}
