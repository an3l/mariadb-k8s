package k8s

import (
	mariak8gv1alpha1 "github.com/mariadb/mariadb.org-tools/mariadb-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	deployment_name = "-server-deployment"
	labels_name     = "mariadb"
	container_name  = "mariadb"
	secret_suffix   = "-secret"
	secret_key      = "mariadb-root-password"
	port_name       = "mariadb-port"
	service_suffix  = "-server-service"
	service_name    = "mariadb-service"
)

func DesiredDeployment(cr mariak8gv1alpha1.MariaDB) (appsv1.Deployment, error) {
	database := cr.Spec.MariaDB.PodSpec
	mariaImage := database.Image // image can be assigned
	if mariaImage == "" {
		mariaImage = "quay.io/mariadb-foundation/mariadb-devel:" + database.ImageVersion // get the latest image version
	}

	mariaPort := database.Port
	if mariaPort == 0 {
		mariaPort = 3306
	}

	// Create the deployment
	depl := appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{APIVersion: appsv1.SchemeGroupVersion.String(), Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + deployment_name,
			Namespace: cr.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: database.Replicas, // won't be nil because defaulting
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{labels_name: cr.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{labels_name: cr.Name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  container_name,
							Image: mariaImage,
							Env: []corev1.EnvVar{
								{
									Name:  "MARIADB_ALLOW_EMPTY_ROOT_PASSWORD",
									Value: "0",
								},
								// ValueFrom cannot be used if Value is non empty
								{
									Name:  "MARIADB_ROOT_PASSWORD",
									Value: database.Rootpwd,
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: cr.Name + secret_suffix,
											},
											Key: secret_key,
										},
									},
								},
								{Name: "MARIADB_USER", Value: database.Username},
								{Name: "MARIADB_PASSWORD", Value: database.Password},
								{Name: "MARIADB_DATABASE", Value: database.Database},
							},
							Ports: []corev1.ContainerPort{
								{ContainerPort: mariaPort, Name: port_name, Protocol: "TCP"},
							},
							//Resources:
						},
					},
				},
			},
		},
	}

	return depl, nil
}

func DesiredService(database mariak8gv1alpha1.MariaDB) (corev1.Service, error) {
	svc := corev1.Service{
		TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "Service"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      service_name,
			Namespace: database.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Name: service_name, Port: database.Spec.MariaDB.Port, Protocol: "TCP", TargetPort: intstr.FromString(service_name)},
			},
			Selector: map[string]string{labels_name: database.Name},
			Type:     corev1.ServiceTypeClusterIP,
		},
	}

	return svc, nil
}

func CreateRootSecret(database *mariak8gv1alpha1.MariaDB, secretPassword string) (corev1.Secret, error) {
	rootSecret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      database.Name + secret_suffix,
			Namespace: database.Namespace,
		},
		Type: "Opaque", // default
		Data: map[string][]byte{
			secret_key: []byte(secretPassword),
		},
	}

	return rootSecret, nil
}
