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
	"github.com/mariadb/mariadb.org-tools/mariadb-operator/pkg/k8s"
	"github.com/mariadb/mariadb.org-tools/mariadb-operator/pkg/mariadb"
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
	/*
		----------- SETTING THE DEFAULTS FOR RECONCILIATION -----------
	*/
	// Create the object to reconcile
	var app mariak8gv1alpha1.MariaDB // fetch resource
	log := r.Log.WithValues("MariaDB: ", req.NamespacedName)
	ctrl_res := ctrl.Result{}

	log.Info("Reconciling MariaDB kind", "mariadb", app.Name)
	// First we should set default versions TODO
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		// it might be not found if this is a delete request
		if ignoreNotFound(err) == nil {
			log.Info("Reconciled MariaDB kind after delete")
			return ctrl_res, nil
		}
		log.Error(err, "unable to fetch MariaDB")
		return ctrl_res, err
	}

	applyOpts := []client.PatchOption{client.ForceOwnership, client.FieldOwner("mariadb-controller")}

	/*
		----------- SETTING THE ROOT SECRET  -----------
	*/
	// Check if secret exists (one cannot specify plain text rootpwd)
	root_secret := &corev1.Secret{}
	err := r.Get(ctx, client.ObjectKey{
		Namespace: req.Namespace,
		Name:      req.Name + "-secret",
	}, root_secret)
	// log.Info("Get()", "error: ", err, "root_secret", *root_secret)
	if err != nil {
		log.Info("Root secret doesn't exist, let me creat it:...")
		// Create root secret if not exist root password
		var err error
		root_secret, err = r.reconcile_root_secret(ctx, app, log)
		err = r.Client.Create(ctx, root_secret)
		if err != nil {
			log.Error(err, " failed to reconcile root secret!")
			return ctrl_res, err
		}

		// Secret created successfully - requeue after 5 minutes
		log.Info("Secret Created successfully, RequeueAfter 5 sec")
		return ctrl.Result{RequeueAfter: 5}, nil
	}

	/*
		----------- SETTING THE CONFIGMAP  -----------
	*/
	cm, err := r.reconcile_configmap(ctx, app, log)
	// return if there is an error during service start
	if err != nil {
		log.Error(err, " failed to reconcile headless service!")
		return ctrl_res, err
	}

	/*
		----------- SETTING THE HEADLESS SERVICE  -----------
	*/
	svc, err := r.reconcile_headless_service(ctx, app, log)
	// return if there is an error during service start
	if err != nil {
		log.Error(err, " failed to reconcile headless service!")
		return ctrl_res, err
	}

	/*
		----------- SETTING THE STATEFULSET  -----------
	*/
	// Create the deployment
	mariadb_sts, err := r.reconcile_statefulset(ctx, app, log)
	if err != nil {
		log.Error(err, " failed to reconcile statefulset!")
		return ctrl_res, err
	}

	/*
		----------- UPDATE INFORMATION IN CLUSTER  -----------
	*/

	err = r.Patch(ctx, &cm, client.Apply, applyOpts...)
	if err != nil {
		return ctrl_res, err
	}

	err = r.Patch(ctx, mariadb_sts, client.Apply, applyOpts...)
	if err != nil {
		return ctrl_res, err
	}

	err = r.Patch(ctx, &svc, client.Apply, applyOpts...)
	if err != nil {
		return ctrl_res, err
	}

	if err := r.Status().Update(ctx, &app); err != nil {
		log.Error(err, "unable to update the variable status")
		return ctrl_res, err
	}

	log.Info("Reconciled MariaDB kind", "mariadb", app.Name, "status", app.Status)

	return ctrl_res, nil
}

/*
	--------------------------------------------------------
	Custom function used to be called in reconciliation loop
	--------------------------------------------------------
*/
func (r *MariaDBReconciler) reconcile_root_secret(
	ctx context.Context,
	app mariak8gv1alpha1.MariaDB,
	log logr.Logger,
) (*corev1.Secret, error) {
	// If not create the secret with password `mysecret`
	root_secret, err_secret := k8s.CreateRootSecret(&app, "mysecret")
	return &root_secret, err_secret
}

func (r *MariaDBReconciler) reconcile_deployment(
	ctx context.Context,
	app mariak8gv1alpha1.MariaDB,
	log logr.Logger,
) (appsv1.Deployment, error) {
	deployment, err := k8s.DesiredDeployment(app)
	// always set the controller reference so that we know which object owns this.
	if err := ctrl.SetControllerReference(&app, &deployment, r.Scheme); err != nil {
		log.Error(err, "unable to set controller reference of deployment")
		return deployment, err
	}
	return deployment, err
}

func (r *MariaDBReconciler) reconcile_service(
	ctx context.Context,
	app mariak8gv1alpha1.MariaDB,
	log logr.Logger,
) (corev1.Service, error) {
	srv, err := k8s.DesiredService(app)
	// always set the controller reference so that we know which object owns this.
	if err := ctrl.SetControllerReference(&app, &srv, r.Scheme); err != nil {
		log.Error(err, "unable to set controller reference of service")
		return srv, err
	}
	return srv, err
}

func (r *MariaDBReconciler) reconcile_statefulset(
	ctx context.Context,
	app mariak8gv1alpha1.MariaDB,
	log logr.Logger,
) (client.Object, error) {
	mariadb_sts_object := mariadb.StatefulSet(app)
	// always set the controller reference so that we know which object owns this.
	if err := ctrl.SetControllerReference(&app, mariadb_sts_object, r.Scheme); err != nil {
		log.Error(err, "unable to set controller reference of statefulset")
		return mariadb_sts_object, err
	}
	return mariadb_sts_object, nil
}

func (r *MariaDBReconciler) reconcile_headless_service(
	ctx context.Context,
	app mariak8gv1alpha1.MariaDB,
	log logr.Logger,
) (corev1.Service, error) {
	srv, err := mariadb.HeadlessService(app)
	// always set the controller reference so that we know which object owns this.
	if err := ctrl.SetControllerReference(&app, &srv, r.Scheme); err != nil {
		log.Error(err, "unable to set controller reference of headless service")
		return srv, err
	}
	return srv, err
}

func (r *MariaDBReconciler) reconcile_configmap(
	ctx context.Context,
	app mariak8gv1alpha1.MariaDB,
	log logr.Logger,
) (corev1.ConfigMap, error) {
	cm := mariadb.ConfigMap(app)
	// always set the controller reference so that we know which object owns this.
	if err := ctrl.SetControllerReference(&app, cm, r.Scheme); err != nil {
		log.Error(err, "unable to set controller reference of configuration map")
		return *cm, err
	}
	return *cm, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MariaDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mariak8gv1alpha1.MariaDB{}).
		Owns(&corev1.Service{}).
		//Owns(&appsv1.Deployment{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
