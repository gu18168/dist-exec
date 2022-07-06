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
	"os"
	"os/exec"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	execv1 "github.com/gu18168/dist-exe/api/v1"
)

// DistExecReconciler reconciles a DistExec object
type DistExecReconciler struct {
	client.Client
	Scheme *runtime.Scheme
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

	// The Controller gets the executing Node name from the environment variable.
	// If the environment variable does not exist, node name is often the same as the hostname.
	nodeName := os.Getenv("MY_NODE_NAME")
	if nodeName == "" {
		nodeName, _ = os.Hostname()
	}

	distExec, err := r.getDistExec(ctx, req, logger)
	if distExec == nil || err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Start to reconcile", "version", distExec.ResourceVersion)

	// Execute the command in the current Node
	command := strings.Split(distExec.Spec.Command, " ")
	execStdout, execStderr, err := r.execInNode(command)
	if err != nil {
		logger.Error(err, "Fail to execute in node",
			"command", distExec.Spec.Command, "node", nodeName)
		return ctrl.Result{}, err
	}

	result := execStdout
	if execStderr != "" {
		result = execStderr
	}

	for {
		targetStatus := distExec.Status.DeepCopy().Results
		if targetStatus == nil {
			targetStatus = make(map[string]string)
		}

		// Update DistExec's status if necessary
		needUpdate := r.newStatus(targetStatus, nodeName, result)
		if !needUpdate {
			logger.Info("No need to update", "version", distExec.ResourceVersion)
			break
		}

		logger.Info("Start to update", "status", targetStatus, "version", distExec.ResourceVersion)
		distExec.Status.Results = targetStatus
		err := r.Status().Update(ctx, distExec)
		if err == nil {
			logger.Info("Update done")
			break
		} else if !errors.IsConflict(err) {
			logger.Error(err, "Fail to update DistExec name", "name", req.NamespacedName)
			return ctrl.Result{}, err
		}

		// Multiple Controllers may start to update at the same time.
		// Kubernetes ensures concurrency security with CAS (metadata.resourceVersion).
		logger.Info("Try to update again")
		distExec, err = r.getDistExec(ctx, req, logger)
		if distExec == nil || err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *DistExecReconciler) getDistExec(ctx context.Context, req ctrl.Request, logger logr.Logger) (*execv1.DistExec, error) {
	var distExec execv1.DistExec
	if err := r.Get(ctx, req.NamespacedName, &distExec); err != nil && errors.IsNotFound(err) {
		logger.Info("DistExec has been deleted")
		return nil, nil
	} else if err != nil {
		logger.Error(err, "Fail to find DistExec", "name", req.NamespacedName)
		return nil, err
	}

	return &distExec, nil
}

// Since the Controller is running in its own Pod, the command is executed directly.
func (r *DistExecReconciler) execInNode(command []string) (string, string, error) {
	cmd := exec.Command(command[0], command[1:]...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	err := cmd.Run()
	if err != nil {
		return "", "", err
	}

	return stdout.String(), stderr.String(), nil
}

// Generate a new status after the execution and check the necessity of the update.
func (r *DistExecReconciler) newStatus(status map[string]string, key, value string) bool {
	needUpdate := false
	if oldValue, ok := status[key]; !ok || value != oldValue {
		status[key] = value
		needUpdate = true
	}

	return needUpdate
}

// SetupWithManager sets up the controller with the Manager.
func (r *DistExecReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&execv1.DistExec{}).
		Complete(r)
}
