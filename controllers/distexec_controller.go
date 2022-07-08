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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/go-logr/logr"
	execv1 "github.com/gu18168/dist-exe/api/v1"
)

// Cache the command that have been executed for each DistExec
var cache = make(map[string]string)

// DistExecReconciler reconciles a DistExec object
type DistExecReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=exec.yuhong.test,resources=distexecs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=exec.yuhong.test,resources=distexecs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=exec.yuhong.test,resources=distexecs/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=nodes,verbs=list;watch

func (r *DistExecReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// The Controller gets the executing Node name from the environment variable.
	// If the environment variable does not exist, node name is often the same as the hostname.
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		nodeName, _ = os.Hostname()
	}

	distExec, err := r.getDistExec(ctx, req.NamespacedName, logger)
	if distExec == nil {
		// If DistExec is deleted, the cache also needs to be deleted.
		if err == nil {
			delete(cache, req.String())
		}
		return ctrl.Result{}, err
	}

	logger.Info("Start to reconcile", "version", distExec.ResourceVersion)

	// Reconcile can be triggered by an update of Status.
	// And `cache` records the last command executed in this Node.
	// If the command remains unchanged, there is no need to re-execute the command.
	if execCommand, ok := cache[req.String()]; ok && execCommand == distExec.Spec.Command {
		logger.Info("No need to execute", "version", distExec.ResourceVersion)
		return ctrl.Result{}, nil
	}

	// Execute the command in the current Node
	command := strings.Split(distExec.Spec.Command, " ")
	execStdout, execStderr, err := r.execInNode(command)
	if err != nil {
		logger.Error(err, "Fail to execute in node",
			"command", distExec.Spec.Command, "node", nodeName)
		return ctrl.Result{}, err
	}

	cache[req.String()] = distExec.Spec.Command

	result := execStdout
	if execStderr != "" {
		result = execStderr
	}

	err = r.updateStatus(ctx, logger, distExec, req.NamespacedName,
		func(distExec *execv1.DistExec) (map[string]string, bool) {
			needUpdate := false
			targetStatus := distExec.Status.DeepCopy().Results
			if targetStatus == nil {
				targetStatus = make(map[string]string)
			}

			if oldValue, ok := targetStatus[nodeName]; !ok || result != oldValue {
				targetStatus[nodeName] = result
				needUpdate = true
			}

			return targetStatus, needUpdate
		})

	return ctrl.Result{}, err
}

func (r *DistExecReconciler) getDistExec(ctx context.Context, name types.NamespacedName, logger logr.Logger) (*execv1.DistExec, error) {
	var distExec execv1.DistExec
	if err := r.Get(ctx, name, &distExec); err != nil && errors.IsNotFound(err) {
		logger.Info("DistExec has been deleted")
		return nil, nil
	} else if err != nil {
		logger.Error(err, "Fail to find DistExec", "name", name)
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

// Due to competing modifications of multiple controllers,
// it is necessary to ensure that the update is successful.
func (r *DistExecReconciler) updateStatus(ctx context.Context, logger logr.Logger,
	distExec *execv1.DistExec, namespacedName types.NamespacedName,
	newStatus func(*execv1.DistExec) (map[string]string, bool)) error {
	for {
		// Update DistExec's status if necessary
		targetStatus, needUpdate := newStatus(distExec)
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
			logger.Error(err, "Fail to update DistExec name", "name", namespacedName)
			return err
		}

		// Multiple Controllers may start to update at the same time.
		// Kubernetes ensures concurrency security with CAS (metadata.resourceVersion).
		logger.Info("Try to update again")
		distExec, err = r.getDistExec(ctx, namespacedName, logger)
		if distExec == nil || err != nil {
			return err
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DistExecReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&execv1.DistExec{}).
		Watches(&source.Kind{Type: &corev1.Node{}}, handler.Funcs{DeleteFunc: r.nodeDeleteHandler}).
		Complete(r)
}

// If a node exits, the Controller on the other node helps maintain the status.
func (r *DistExecReconciler) nodeDeleteHandler(e event.DeleteEvent, q workqueue.RateLimitingInterface) {
	ctx := context.Background()
	logger := log.FromContext(ctx)

	logger.Info("Start to node delete handler")

	var distExecList execv1.DistExecList
	if err := r.List(ctx, &distExecList); err != nil && errors.IsNotFound(err) {
		logger.Info("No DistExec is found")
		return
	} else if err != nil {
		logger.Error(err, "Fail to find DistExec")
		return
	}

	nodeName := e.Object.GetName()
	statusUpdater := func(distExec *execv1.DistExec) (map[string]string, bool) {
		if _, ok := distExec.Status.Results[nodeName]; !ok {
			return nil, false
		}

		targetStatus := distExec.Status.DeepCopy().Results
		delete(targetStatus, nodeName)
		return targetStatus, true
	}

	for _, obj := range distExecList.Items {
		distExec := &obj
		namespacedName := types.NamespacedName{
			Namespace: distExec.GetNamespace(),
			Name:      distExec.GetName(),
		}

		_ = r.updateStatus(ctx, logger, distExec, namespacedName, statusUpdater)
	}
}
