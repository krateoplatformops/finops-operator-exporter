package utils

import (
	"context"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	finopsdatatypes "github.com/krateoplatformops/finops-data-types/api/v1"
	finopsv1 "github.com/krateoplatformops/finops-operator-exporter/api/v1"
	secretHelper "github.com/krateoplatformops/finops-operator-exporter/internal/helpers/kube/secrets"
)

func Int32Ptr(i int32) *int32 { return &i }

func GetGenericExporterDeployment(exporterScraperConfig *finopsv1.ExporterScraperConfig) (*appsv1.Deployment, error) {
	imageName := strings.TrimSuffix(os.Getenv("REGISTRY"), "/")
	imageVersion := strings.TrimSuffix(os.Getenv("EXPORTER_VERSION"), "latest")
	image := strings.TrimSuffix(os.Getenv("EXPORTER_NAME"), "finops-prometheus-exporter")

	imageName += "/" + image + ":" + imageVersion

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      exporterScraperConfig.Name + "-deployment",
			Namespace: exporterScraperConfig.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: exporterScraperConfig.APIVersion,
					Kind:       exporterScraperConfig.Kind,
					Name:       exporterScraperConfig.Name,
					UID:        exporterScraperConfig.UID,
				},
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: Int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"scraper": exporterScraperConfig.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"scraper": exporterScraperConfig.Name,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "exporterscraper-config-getter-sa",
					Containers: []corev1.Container{
						{
							Name:            "scraper",
							Image:           imageName,
							ImagePullPolicy: corev1.PullAlways,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config-volume",
									MountPath: "/config",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config-volume",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: exporterScraperConfig.Name + "-configmap",
									},
								},
							},
						},
					},
					ImagePullSecrets: []corev1.LocalObjectReference{
						{
							Name: os.Getenv("REGISTRY_CREDENTIALS"),
						},
					},
				},
			},
		},
	}
	deployment.APIVersion = "apps/v1"
	return deployment, nil
}

func GetGenericExporterConfigMap(exporterScraperConfigComplete *finopsv1.ExporterScraperConfig) (*corev1.ConfigMap, error) {
	exporterScraperConfig := &finopsv1.ExporterScraperConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      exporterScraperConfigComplete.ObjectMeta.Name,
			Namespace: exporterScraperConfigComplete.ObjectMeta.Namespace,
			UID:       exporterScraperConfigComplete.ObjectMeta.UID,
		},
		TypeMeta: exporterScraperConfigComplete.TypeMeta,
		Spec:     exporterScraperConfigComplete.Spec,
	}
	yamlData, err := yaml.Marshal(exporterScraperConfig)
	if err != nil {
		return &corev1.ConfigMap{}, err
	}

	binaryData := make(map[string][]byte)
	binaryData["config.yaml"] = yamlData
	configmap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      exporterScraperConfig.Name + "-configmap",
			Namespace: exporterScraperConfig.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: exporterScraperConfig.APIVersion,
					Kind:       exporterScraperConfig.Kind,
					Name:       exporterScraperConfig.Name,
					UID:        exporterScraperConfig.UID,
				},
			},
		},
		BinaryData: binaryData,
	}
	configmap.APIVersion = "v1"
	return configmap, nil
}

func GetGenericExporterService(exporterScraperConfig *finopsv1.ExporterScraperConfig) (*corev1.Service, error) {
	labels := make(map[string]string)
	labels["scraper"] = exporterScraperConfig.Name
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      exporterScraperConfig.Name + "-service",
			Namespace: exporterScraperConfig.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: exporterScraperConfig.APIVersion,
					Kind:       exporterScraperConfig.Kind,
					Name:       exporterScraperConfig.Name,
					UID:        exporterScraperConfig.UID,
				},
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Type:     corev1.ServiceTypeNodePort,
			Ports: []corev1.ServicePort{
				{
					Protocol: corev1.ProtocolTCP,
					Port:     2112,
				},
			},
		},
	}
	service.APIVersion = "v1"
	return service, nil
}

func GetGenericExporterScraperConfig(exporterScraperConfig *finopsv1.ExporterScraperConfig, serviceIp string, servicePort int) (*finopsdatatypes.ScraperConfig, error) {
	if exporterScraperConfig.Spec.ScraperConfig.TableName == "" &&
		exporterScraperConfig.Spec.ScraperConfig.MetricType == "" &&
		exporterScraperConfig.Spec.ScraperConfig.PollingInterval.Seconds() == 0 &&
		exporterScraperConfig.Spec.ScraperConfig.ScraperDatabaseConfigRef.Name == "" &&
		exporterScraperConfig.Spec.ScraperConfig.ScraperDatabaseConfigRef.Namespace == "" {
		return nil, nil
	}

	var api finopsdatatypes.API
	if exporterScraperConfig.Spec.ScraperConfig.API.EndpointRef == nil {
		url := "http://" + serviceIp + ":" + strconv.FormatInt(int64(servicePort), 10)
		err := createEndpointRef(exporterScraperConfig, url)
		if err != nil {
			return nil, err
		}
		api = finopsdatatypes.API{
			Path: "/metrics",
			Verb: "GET",
			EndpointRef: &finopsdatatypes.ObjectRef{
				Name:      exporterScraperConfig.Name,
				Namespace: exporterScraperConfig.Namespace,
			},
		}
	} else {
		api = exporterScraperConfig.Spec.ScraperConfig.API
	}

	return &finopsdatatypes.ScraperConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ScraperConfig",
			APIVersion: "finops.krateo.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      exporterScraperConfig.Name + "-scraper",
			Namespace: exporterScraperConfig.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: exporterScraperConfig.APIVersion,
					Kind:       exporterScraperConfig.Kind,
					Name:       exporterScraperConfig.Name,
					UID:        exporterScraperConfig.UID,
				},
			},
		},
		Spec: finopsdatatypes.ScraperConfigSpec{
			TableName:       exporterScraperConfig.Spec.ScraperConfig.TableName,
			API:             api,
			MetricType:      exporterScraperConfig.Spec.ExporterConfig.MetricType,
			PollingInterval: exporterScraperConfig.Spec.ScraperConfig.PollingInterval,
			ScraperDatabaseConfigRef: finopsdatatypes.ObjectRef{
				Name:      exporterScraperConfig.Spec.ScraperConfig.ScraperDatabaseConfigRef.Name,
				Namespace: exporterScraperConfig.Spec.ScraperConfig.ScraperDatabaseConfigRef.Namespace,
			},
		},
	}, nil
}

func createEndpointRef(exporterScraperConfig *finopsv1.ExporterScraperConfig, url string) error {
	secretToCreate := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      exporterScraperConfig.Name,
			Namespace: exporterScraperConfig.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: exporterScraperConfig.APIVersion,
					Kind:       exporterScraperConfig.Kind,
					Name:       exporterScraperConfig.Name,
					UID:        exporterScraperConfig.UID,
				},
			},
		},
		StringData: map[string]string{
			"server-url": url,
		},
	}

	return secretHelper.CreateOrUpdate(context.TODO(), secretToCreate)
}

type CRDResponse struct {
	Status string `json:"status"`
}
