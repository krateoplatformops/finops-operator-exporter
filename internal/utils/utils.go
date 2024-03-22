package utils

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	finopsv1 "operator-exporter/api/v1"
)

var repository = os.Getenv("REPO")

func Int32Ptr(i int32) *int32 { return &i }

func GetGenericExporterDeployment(exporterScraperConfig finopsv1.ExporterScraperConfig) (*appsv1.Deployment, error) {
	imageName := repository
	if strings.Contains(exporterScraperConfig.Spec.ExporterConfig.URL, "@RES:") {
		imageName += "/prometheus-resource-exporter-azure:0.1"
	} else {
		imageName += "/prometheus-exporter-generic:0.1"
	}

	return &appsv1.Deployment{
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
							Env: []corev1.EnvVar{
								{
									Name: "NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
								{
									Name: "DEPLOYMENT",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
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
							Name: "registry-credentials-default",
						},
					},
				},
			},
		},
	}, nil
}

func GetGenericExporterConfigMap(exporterScraperConfig finopsv1.ExporterScraperConfig) (*corev1.ConfigMap, error) {
	exporterScraperConfig.Spec.ExporterConfig.URL = strings.Replace(exporterScraperConfig.Spec.ExporterConfig.URL, "@RES:", "", 1)

	yamlData, err := yaml.Marshal(exporterScraperConfig.Spec.ExporterConfig)
	if err != nil {
		return &corev1.ConfigMap{}, err
	}

	binaryData := make(map[string][]byte)
	binaryData["config.yaml"] = yamlData
	return &corev1.ConfigMap{
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
	}, nil
}

func GetGenericExporterService(exporterScraperConfig finopsv1.ExporterScraperConfig) (*corev1.Service, error) {
	labels := make(map[string]string)
	labels["scraper"] = exporterScraperConfig.Name
	return &corev1.Service{
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
	}, nil
}

func CreateScraperCR(ctx context.Context, exporterScraperConfig finopsv1.ExporterScraperConfig, serviceIp string, servicePort int) error {
	inClusterConfig, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(inClusterConfig)
	if err != nil {
		return err
	}

	jsonData, _ := clientset.RESTClient().Get().
		AbsPath("/apis/finops.krateo.io/v1").
		Namespace(exporterScraperConfig.Namespace).
		Resource("scraperconfigs").
		Name(exporterScraperConfig.Name + "-scraper").
		DoRaw(context.TODO())

	var crdResponse CRDResponse
	_ = json.Unmarshal(jsonData, &crdResponse)
	if crdResponse.Status == "Failure" {
		url := exporterScraperConfig.Spec.ScraperConfig.Url
		if url == "" {
			url = "http://" + serviceIp + ":" + strconv.FormatInt(int64(servicePort), 10) + "/metrics"
		}
		scraperConfig := &finopsv1.ScraperConfigFromScraperOperator{
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
			Spec: finopsv1.ScraperConfigSpecFromScraperOperator{
				TableName:            exporterScraperConfig.Spec.ScraperConfig.TableName,
				Url:                  url,
				PollingIntervalHours: exporterScraperConfig.Spec.ScraperConfig.PollingIntervalHours,
				ScraperDatabaseConfigRef: finopsv1.ScraperDatabaseConfigRef{
					Name:      exporterScraperConfig.Spec.ScraperConfig.ScraperDatabaseConfigRef.Name,
					Namespace: exporterScraperConfig.Spec.ScraperConfig.ScraperDatabaseConfigRef.Namespace,
				},
			},
		}
		jsonData, err = json.Marshal(scraperConfig)
		if err != nil {
			return err
		}
		_, err := clientset.RESTClient().Post().
			AbsPath("/apis/finops.krateo.io/v1").
			Namespace(exporterScraperConfig.Namespace).
			Resource("scraperconfigs").
			Name(exporterScraperConfig.Name).
			Body(jsonData).
			DoRaw(ctx)

		if err != nil {
			return err
		}
	}
	return nil
}

type CRDResponse struct {
	Status string `json:"status"`
}
