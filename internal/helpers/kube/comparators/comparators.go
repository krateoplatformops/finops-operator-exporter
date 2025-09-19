package comparators

import (
	"os"
	"strings"

	finopsv1 "github.com/krateoplatformops/finops-operator-exporter/api/v1"
	operatorscraperapi "github.com/krateoplatformops/finops-operator-scraper/api/v1"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CheckService(service corev1.Service, exporterScraperConfig finopsv1.ExporterScraperConfig) bool {
	if service.Name != exporterScraperConfig.Name+"-service" {
		return false
	}

	ownerReferencesLive := service.OwnerReferences
	if len(ownerReferencesLive) != 1 {
		log.Logger.Debug().Msg("Owner reference length not one")
		return false
	}

	if service.Spec.Selector["scraper"] != exporterScraperConfig.Name {
		return false
	}

	if service.Spec.Type != corev1.ServiceTypeNodePort {
		log.Logger.Debug().Msg("service type wrong")
		return false
	}

	if len(service.Spec.Ports) == 0 {
		log.Logger.Debug().Msg("service ports number wrong")
		return false
	} else {
		found := false
		for _, port := range service.Spec.Ports {
			if port.Protocol == corev1.ProtocolTCP && port.Port == 2112 {
				found = true
			}
		}
		if !found {
			log.Logger.Debug().Msg("service port range wrong")
			return false
		}
	}

	if ownerReferencesLive[0].Kind != exporterScraperConfig.Kind ||
		ownerReferencesLive[0].Name != exporterScraperConfig.Name ||
		ownerReferencesLive[0].UID != exporterScraperConfig.UID ||
		ownerReferencesLive[0].APIVersion != exporterScraperConfig.APIVersion {
		log.Logger.Debug().Msg("Owner reference wrong")
		return false
	}

	return true
}

func CheckConfigMap(configMap corev1.ConfigMap, exporterScraperConfigComplete finopsv1.ExporterScraperConfig) bool {
	if configMap.Name != exporterScraperConfigComplete.Name+"-configmap" {
		log.Logger.Debug().Msg("name wrong")
		return false
	}

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
		return false
	}

	if yamlDataFromLive, ok := configMap.BinaryData["config.yaml"]; ok {
		if strings.Replace(string(yamlData), " ", "", -1) != strings.Replace(string(yamlDataFromLive), " ", "", -1) {
			log.Logger.Debug().Msg("bytes different")
			log.Logger.Debug().Msg(string(yamlDataFromLive))
			log.Logger.Debug().Msg(string(yamlData))
			return false
		}
	} else {
		log.Logger.Debug().Msg("config.yaml not in map")
		return false
	}

	ownerReferencesLive := configMap.OwnerReferences
	if len(ownerReferencesLive) != 1 {
		log.Logger.Debug().Msg("Owner reference length not one")
		return false
	}

	if ownerReferencesLive[0].Kind != exporterScraperConfig.Kind ||
		ownerReferencesLive[0].Name != exporterScraperConfig.Name ||
		ownerReferencesLive[0].UID != exporterScraperConfig.UID ||
		ownerReferencesLive[0].APIVersion != exporterScraperConfig.APIVersion {
		log.Logger.Debug().Msg("owner reference wrong")
		return false
	}

	return true
}

func CheckDeployment(deployment appsv1.Deployment, exporterScraperConfig finopsv1.ExporterScraperConfig) bool {
	if deployment.Name != exporterScraperConfig.Name+"-deployment" {
		log.Logger.Debug().Msg("Name does not respect naming convention")
		return false
	}

	ownerReferencesLive := deployment.OwnerReferences
	if len(ownerReferencesLive) != 1 {
		log.Logger.Debug().Msg("Owner reference length not one")
		return false
	}

	if ownerReferencesLive[0].Kind != exporterScraperConfig.Kind ||
		ownerReferencesLive[0].Name != exporterScraperConfig.Name ||
		ownerReferencesLive[0].UID != exporterScraperConfig.UID ||
		ownerReferencesLive[0].APIVersion != exporterScraperConfig.APIVersion {
		log.Logger.Debug().Msg("Owner reference wrong")
		return false
	}

	if *deployment.Spec.Replicas != 1 {
		log.Logger.Debug().Msgf("Replicas not one: %s = %d", "replicas", deployment.Spec.Replicas)
		return false
	}

	if len(deployment.Spec.Selector.MatchLabels) == 0 {
		log.Logger.Debug().Msg("Selector not found")
		return false
	} else if deployment.Spec.Selector.MatchLabels["scraper"] != exporterScraperConfig.Name {
		log.Logger.Debug().Msg("Selector label scraper not equal to config name")
		return false
	}

	if len(deployment.Spec.Template.ObjectMeta.Labels) == 0 {
		log.Logger.Debug().Msg("No labels found")
		return false
	} else if deployment.Spec.Template.ObjectMeta.Labels["scraper"] != exporterScraperConfig.Name {
		log.Logger.Debug().Msg("Label scraper not equal to config name")
		return false
	}

	imageName := strings.TrimSuffix(os.Getenv("REGISTRY"), "/")
	imageVersion := os.Getenv("EXPORTER_VERSION")
	image := os.Getenv("EXPORTER_NAME")

	if imageVersion == "" {
		imageVersion = "latest"
	}
	if image == "" {
		image = "finops-prometheus-exporter"
	}

	imageName += "/" + image + ":" + imageVersion

	if len(deployment.Spec.Template.Spec.Containers) != 1 {
		log.Logger.Debug().Msg("Container not equal to 1")
		return false
	} else {
		if deployment.Spec.Template.Spec.Containers[0].Image != imageName {
			log.Logger.Debug().Msgf("Image name/version not matching: found %s, want %s", deployment.Spec.Template.Spec.Containers[0].Image, imageName)
			return false
		}

		if len(deployment.Spec.Template.Spec.Containers[0].VolumeMounts) == 0 {
			log.Logger.Debug().Msg("No volume mount found")
			return false
		} else {
			found := false
			for _, volumeMount := range deployment.Spec.Template.Spec.Containers[0].VolumeMounts {
				if volumeMount.Name == "config-volume" && volumeMount.MountPath == "/config" {
					found = true
				}
			}
			if !found {
				log.Logger.Debug().Msg("Volume mount not found")
				return false
			}
		}

		if len(deployment.Spec.Template.Spec.Volumes) == 0 {
			log.Logger.Debug().Msg("No volumes found")
			return false
		} else {
			found := false
			for _, volume := range deployment.Spec.Template.Spec.Volumes {
				if volume.Name == "config-volume" && volume.VolumeSource.ConfigMap.LocalObjectReference.Name == exporterScraperConfig.Name+"-configmap" {
					found = true
				}
			}
			if !found {
				log.Logger.Debug().Msg("Volume not found")
				return false
			}
		}
	}

	// Container image and secret name are not checked on purpose, since they may need to be different from the default values

	return true
}

func CheckScraper(scraperObj operatorscraperapi.ScraperConfig, exporterScraperConfig finopsv1.ExporterScraperConfig) bool {
	s1 := exporterScraperConfig.Spec.ScraperConfig
	s2 := scraperObj.Spec

	if s1.TableName != s2.TableName ||
		s1.PollingInterval.Seconds() != s2.PollingInterval.Seconds() ||
		exporterScraperConfig.Spec.ExporterConfig.MetricType != s2.MetricType {
		log.Debug().Msgf("Basic fields not equal: expected %+v, got %+v", s1, s2)
		return false
	}

	if s1.ScraperDatabaseConfigRef.Name != s2.ScraperDatabaseConfigRef.Name ||
		s1.ScraperDatabaseConfigRef.Namespace != s2.ScraperDatabaseConfigRef.Namespace {
		log.Debug().Msgf("Database config ref not equal: expected %+v, got %+v", s1.ScraperDatabaseConfigRef, s2.ScraperDatabaseConfigRef)
		return false
	}

	if s2.API.Path != "/metrics" ||
		s2.API.Verb != "GET" {
		log.Debug().Msgf("API fields not equal: expected %+v, got %+v", s1.API, s2.API)
		return false
	}

	if exporterScraperConfig.Name != s2.API.EndpointRef.Name ||
		exporterScraperConfig.Namespace != s2.API.EndpointRef.Namespace {
		log.Debug().Msgf("API EndpointRef not equal: expected %s %s, got %+v", exporterScraperConfig.Name, exporterScraperConfig.Namespace, s2.API.EndpointRef)
		return false
	}

	if len(s2.API.Headers) != 0 {
		log.Debug().Msgf("API Headers length mismatch: expected %d, got %d", 0, len(s2.API.Headers))
		return false
	}

	return true
}
