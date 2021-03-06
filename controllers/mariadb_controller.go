/*
Copyright 2021.

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
	"context"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mariak8gv1alpha1 "github.com/mariadb/mariadb.org-tools/mariadb-operator/api/v1alpha1"
)

func ignoreNotFound(err error) error {
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

// MariaDBReconciler reconciles a MariaDB object
type MariaDBReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=mariak8g.mariadb.org,resources=mariadbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mariak8g.mariadb.org,resources=mariadbs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mariak8g.mariadb.org,resources=mariadbs/finalizers,verbs=update

func (r *MariaDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	log := r.Log.WithValues("MariaDB: ", req.NamespacedName)

	var app mariak8gv1alpha1.MariaDB // fetch resource
	log.Info("Reconciling MariaDB kind", "mariadb", app.Name)

	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		// it might be not found if this is a delete request
		if ignoreNotFound(err) == nil {
			log.Info("Reconciled MariaDB kind after delete")
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch MariaDB")
		return ctrl.Result{}, err
	}

	deployment, err := r.desiredDeployment(app)
	if err == nil {
		app.Status.DbState = mariak8gv1alpha1.RunningStatusPhase
		app.Status.ShowState = string(app.Status.DbState)
	}

	// return if there is an error during deployment start
	if err != nil {
		return ctrl.Result{}, err
	}

	svc, err := r.desiredService(app)
	// return if there is an error during service start
	if err != nil {
		return ctrl.Result{}, err
	}

	applyOpts := []client.PatchOption{client.ForceOwnership, client.FieldOwner("mariadb-controller")}

	err = r.Patch(ctx, &deployment, client.Apply, applyOpts...)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.Patch(ctx, &svc, client.Apply, applyOpts...)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.Status().Update(ctx, &app); err != nil {
		log.Error(err, "unable to update the variable status")
		return ctrl.Result{}, err
	}

	log.Info("Reconciled MariaDB kind", "mariadb", app.Name, "status", app.Status)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MariaDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mariak8gv1alpha1.MariaDB{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
