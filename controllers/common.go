package controllers

import (
	//"context"

	//"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	//"k8s.io/apimachinery/pkg/api/errors"
	//"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	//"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	//"sigs.k8s.io/controller-runtime/pkg/client"

	mariak8gv1alpha1 "github.com/mariadb/mariadb.org-tools/mariadb-operator/api/v1alpha1"
)

func (r *MariaDBReconciler) desiredDeployment(database mariak8gv1alpha1.MariaDB) (appsv1.Deployment, error) {
	mariaImage := database.Spec.Image // image can be assigned
	if mariaImage == "" {
		mariaImage = "quay.io/mariadb-foundation/mariadb-devel:" + database.Spec.ImageVersion // get the latest image version
	}

	mariaPort := database.Spec.Port
	if mariaPort == 0 {
		mariaPort = 3306
	}

	// Create the deployment
	depl := appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{APIVersion: appsv1.SchemeGroupVersion.String(), Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      database.Name + "-server",
			Namespace: database.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: database.Spec.Replicas, // won't be nil because defaulting
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"mariadb": database.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"mariadb": database.Name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "mariadb",
							Image: mariaImage,
							Env: []corev1.EnvVar{
								{Name: "MARIADB_ALLOW_EMPTY_ROOT_PASSWORD", Value: "1"},
								// root password should be set from secret - test
								{Name: "MARIADB_ROOT_PASSWORD", Value: database.Spec.Rootpwd},
								{Name: "MARIADB_USER", Value: database.Spec.Username},
								{Name: "MARIADB_PASSWORD", Value: database.Spec.Password},
							},
							Ports: []corev1.ContainerPort{
								{ContainerPort: mariaPort, Name: "mariadb-port", Protocol: "TCP"},
							},
							//Resources:
						},
					},
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(&database, &depl, r.Scheme); err != nil {
		return depl, err
	}

	return depl, nil
}

func (r *MariaDBReconciler) desiredService(database mariak8gv1alpha1.MariaDB) (corev1.Service, error) {
	svc := corev1.Service{
		TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "Service"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      database.Name,
			Namespace: database.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Name: "mariadb-service", Port: database.Spec.Port, Protocol: "TCP", TargetPort: intstr.FromString("mariadb-service")},
			},
			Selector: map[string]string{"mariadb": database.Name},
			Type:     corev1.ServiceTypeClusterIP,
		},
	}

	// always set the controller reference so that we know which object owns this.
	if err := ctrl.SetControllerReference(&database, &svc, r.Scheme); err != nil {
		return svc, err
	}

	return svc, nil
}
