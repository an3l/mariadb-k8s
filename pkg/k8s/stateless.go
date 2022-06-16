package k8s

import (
	mariak8gv1alpha1 "github.com/mariadb/mariadb.org-tools/mariadb-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func DesiredDeployment(database mariak8gv1alpha1.MariaDB) (appsv1.Deployment, error) {
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
			Name:      database.Name + "-server-deployment",
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
								{Name: "MARIADB_ALLOW_EMPTY_ROOT_PASSWORD", Value: "0"},
								// ValueFrom cannot be used if Value is non empty
								{
									Name:  "MARIADB_ROOT_PASSWORD",
									Value: database.Spec.Rootpwd,
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: database.Name + "-secret",
											},
											Key: "mariadb-root-password",
										},
									},
								},
								{Name: "MARIADB_USER", Value: database.Spec.Username},
								{Name: "MARIADB_PASSWORD", Value: database.Spec.Password},
								{Name: "MARIADB_DATABASE", Value: database.Spec.Database},
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

	return depl, nil
}

func DesiredService(database mariak8gv1alpha1.MariaDB) (corev1.Service, error) {
	svc := corev1.Service{
		TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "Service"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      database.Name + "-server-service",
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

	return svc, nil
}

func CreateRootSecret(database *mariak8gv1alpha1.MariaDB, secretPassword string) (corev1.Secret, error) {
	rootSecret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      database.Name + "-secret",
			Namespace: database.Namespace,
		},
		Type: "Opaque", // default
		Data: map[string][]byte{
			"mariadb-root-password": []byte(secretPassword),
		},
	}

	return rootSecret, nil
}
