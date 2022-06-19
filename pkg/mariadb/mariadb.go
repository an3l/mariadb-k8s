package mariadb

import (
	mariak8gv1alpha1 "github.com/mariadb/mariadb.org-tools/mariadb-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	sts_name            = "mariadb-sts"
	labels_name         = "mariadb"
	container_name      = "mariadb"
	secret_suffix       = "-secret"
	secret_key          = "mariadb-root-password"
	port_name           = "mariadb-port"
	service_suffix      = "-server-service"
	service_name        = "mariadb-service"
	dataDirName         = "datadir"
	pvcName             = dataDirName
	dataDirMountPath    = "/var/lib/mysql"
	configVolumeName    = "mariadb-config"
	configMountPath     = "/etc/mysql/config.d"
	configMapName       = "mariadb-configmap"
	configMapVolumeName = "mariadb-config-map"
	configMapMountPath  = "/mnt/config-map"
	initVolumeName      = "initdb"
	initMountPath       = "/docker-entrypoint-initdb.d"
	headlessPortName    = "mariadb-headless-port"
)

func ConfigMap(database mariak8gv1alpha1.MariaDB) *corev1.ConfigMap {
	// We could give keys as values in data tags as part of CR
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: database.Namespace,
		},
		Data: map[string]string{
			"primary.cnf": `
			[mariadb]
			log-bin                         # enable binary loging
			# server_id=3000                  # used to uniquely identify the server
			log-basename=my-mariadb         # used to be independent of to hostname changes (otherwise is in datadir/mysql)
			#binlog-format=MIXED            #default
			`,
			"replica.cnf": `
			[mariadb]
    		# server_id=3001                  # used to uniquely identify the server
    		log-basename=my-mariadb         # used to be independent of to hostname changes (otherwise is in datadir/mysql)
			`,
			"primary.sql": `
			CREATE USER 'repluser'@'%' IDENTIFIED BY 'replsecret';
			GRANT REPLICATION SLAVE ON *.* TO 'repluser'@'%';
			CREATE DATABASE primary_db;
			`,
		},
	}
}

// Private member function
func volumeClaimTemplates(database mariak8gv1alpha1.MariaDB) []corev1.PersistentVolumeClaim {
	return []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: pvcName,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				// StorageClassName:  {"default"},
				AccessModes: []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("300M"),
					},
				},
			},
		},
	}
}

func HeadlessService(database mariak8gv1alpha1.MariaDB) (corev1.Service, error) {
	clusterIP := "None"
	serviceType := corev1.ServiceTypeClusterIP

	svc := corev1.Service{
		TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "Service"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      service_name,
			Namespace: database.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     headlessPortName,
					Port:     3306,
					Protocol: "TCP",
				},
			},
			Selector:  map[string]string{labels_name: database.Name},
			Type:      serviceType,
			ClusterIP: clusterIP,
		},
	}

	return svc, nil

}

func StatefulSet(database mariak8gv1alpha1.MariaDB) client.Object {

	spec := database.Spec.MariaDB.PodSpec
	replicas := spec.Replicas
	cm_optional := true

	return &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "StatefulSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      database.Name,
			Namespace: database.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{labels_name: database.Name},
			},
			ServiceName:          service_name,
			VolumeClaimTemplates: volumeClaimTemplates(database),
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{labels_name: database.Name},
				},
				Spec: corev1.PodSpec{
					// In initcontainer we have to copy appropriate files from configMap
					InitContainers: []corev1.Container{
						{
							Name:  container_name + "-init",
							Image: spec.Image,
							Env: []corev1.EnvVar{
								{
									Name:  "MARIADB_ROOT_PASSWORD",
									Value: spec.Rootpwd,
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: database.Name + secret_suffix,
											},
											Key: secret_key,
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      configMapVolumeName,
									MountPath: configMapMountPath,
								},
								{
									Name:      configVolumeName,
									MountPath: configMountPath,
								},
								{
									Name:      initVolumeName,
									MountPath: initMountPath,
								},
							},
							// TODO: find a way to move the commands to the script
							Command: []string{"/bin/bash",
								"-c",
								"set -ex",
								// Check config map to directory that already exists (but must be used as a volume for main container)
								"ls /mnt/config-map",
								// statefulset has sticky identity, number should be last
								"[[ `hostname` =~ -([0-9]+)$ ]] || exit 1",
								"ordinal=${BASH_REMATCH[1]}",
								// Copy appropriate conf.d files from config-map to emptyDir.
								`if [[ $ordinal -eq 0 ]]; then
							  cp /mnt/config-map/primary.cnf /etc/mysql/conf.d/server-id.cnf
							  # Create the users needed for replication on primary
							  cp /mnt/config-map/primary.sql /docker-entrypoint-initdb.d
							else
							  cp /mnt/config-map/replica.cnf /etc/mysql/conf.d/server-id.cnf
							  # We cannot know the IP of the host, it will be created
							  # cp /mnt/config-map/secondary.sql /docker-entrypoint-initdb.d
							fi
							# Add an offset to avoid reserved server-id=0 value.
							echo server-id=$((3000 + $ordinal)) >> etc/mysql/conf.d/server-id.cnf
							# cp /mnt/config-map/mariadb.cnf /etc/mysql/conf.d
							# Execute the script if needed (maybe for galera)
							# ./mnt/config-map/init.sh
							`,
							},
							TerminationMessagePath:   "/dev/termination-log",
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						}, // end of initContainer[0],
					},
					Containers: mariadbdContainer(database),
					// TerminationGracePeriodSeconds: 30,
					RestartPolicy: corev1.RestartPolicyAlways,
					SchedulerName: "default-scheduler",
					DNSPolicy:     corev1.DNSClusterFirst,
					Volumes: append(
						[]corev1.Volume{
							{
								Name: configVolumeName,
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
							{
								Name: initVolumeName,
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
							{
								Name: configMapVolumeName,
								VolumeSource: corev1.VolumeSource{
									Projected: &corev1.ProjectedVolumeSource{
										Sources: []corev1.VolumeProjection{
											{
												ConfigMap: &corev1.ConfigMapProjection{
													LocalObjectReference: corev1.LocalObjectReference{
														Name: configMapName,
													},
													/*
														// Keys will be added as a files with key names and values with content
														// Alternatively, configMap has 3 keys and we could add their values as custom file names
														Items: []corev1.KeyToPath{
															{
																Key:  CustomConfigKey,
																Path: "primary-config.cnf",
															},
														},
													*/
													// ConfigMap or keys must be defined
													Optional: &cm_optional,
												},
											},
											/* 											{
												Secret: &corev1.SecretProjection{
													LocalObjectReference: corev1.LocalObjectReference{
														Name: ConfigMapName(cr),
													},
													Items: []corev1.KeyToPath{
														{
															Key:  CustomConfigKey,
															Path: "my-secret.cnf",
														},
													},
													Optional: &t,
												},
											}, */
										},
									},
								},
							},
						}, // end of VOlume
					),
				},
			},
		},
	}
}

func getIMageNPort(cr mariak8gv1alpha1.MariaDB) (string, int32) {
	database := cr.Spec.MariaDB.PodSpec
	mariaImage := database.Image // image can be assigned
	if mariaImage == "" {
		mariaImage = "quay.io/mariadb-foundation/mariadb-devel:" + database.ImageVersion // get the latest image version
	}
	mariaPort := database.Port
	if mariaPort == 0 {
		mariaPort = 3306
	}
	return mariaImage, mariaPort
}

func mariadbdContainer(database mariak8gv1alpha1.MariaDB) []corev1.Container {
	spec := database.Spec.MariaDB.PodSpec
	mariaImg, mariaPort := getIMageNPort(database)
	return []corev1.Container{
		{
			Name:  sts_name,
			Image: mariaImg,
			Env: []corev1.EnvVar{
				{
					Name:  "MARIADB_ALLOW_EMPTY_ROOT_PASSWORD",
					Value: "0",
				},
				// ValueFrom cannot be used if Value is non empty
				{
					Name:  "MARIADB_ROOT_PASSWORD",
					Value: spec.Rootpwd,
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: database.Name + secret_suffix,
							},
							Key: secret_key,
						},
					},
				},
				{
					Name:  "MARIADB_USER",
					Value: spec.Username,
				},
				{
					Name:  "MARIADB_PASSWORD",
					Value: spec.Password,
				},
				{
					Name:  "MARIADB_DATABASE",
					Value: spec.Database,
				},
				{
					Name:  "MYSQL_INITDB_SKIP_TZINFO",
					Value: "yes",
				},
				// For healthcheck we need to create mysql user
				{
					Name:  "MARIADB_MYSQL_LOCALHOST_USER",
					Value: "1",
				},
			},
			Ports: []corev1.ContainerPort{
				{
					Name:          port_name,
					ContainerPort: mariaPort,
					//Protocol: "TCP"
				},
				// Probably we will need more ports for primary?
				{
					Name:          "mariadb-primary",
					ContainerPort: 33060,
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      dataDirName,
					MountPath: dataDirMountPath,
				},
				{
					Name:      configVolumeName,
					MountPath: configMountPath,
				},
				{
					Name:      initVolumeName,
					MountPath: initMountPath,
				},
			},
			LivenessProbe: &corev1.Probe{
				Handler: corev1.Handler{
					Exec: &corev1.ExecAction{
						Command: []string{"/usr/local/bin/healthcheck.sh --su=mysql --connect --innodb_initialized"},
					},
				},
				InitialDelaySeconds: 3,
				TimeoutSeconds:      3,
				PeriodSeconds:       3,
				//FailureThreshold:
				//SuccessThreshold:
				//TerminationGracePeriodSeconds:
			},
		}, // end of mysqld container
	}
}
