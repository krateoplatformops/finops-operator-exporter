package comparators

import (
	"strings"

	finopsv1 "github.com/krateoplatformops/finops-operator-exporter/api/v1"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func CheckService(service corev1.Service, exporterScraperConfig finopsv1.ExporterScraperConfig) bool {
	if service.Name != exporterScraperConfig.Name+"-service" {
		return false
	}

	ownerReferencesLive := service.OwnerReferences
	if len(ownerReferencesLive) != 1 {
		return false
	}

	if service.Spec.Selector["scraper"] != exporterScraperConfig.Name {
		return false
	}

	if service.Spec.Type != corev1.ServiceTypeNodePort {
		return false
	}

	if len(service.Spec.Ports) == 0 {
		return false
	} else {
		found := false
		for _, port := range service.Spec.Ports {
			if port.Protocol == corev1.ProtocolTCP && port.Port == 2112 {
				found = true
			}
		}
		if !found {
			return false
		}
	}

	if ownerReferencesLive[0].Kind != exporterScraperConfig.Kind ||
		ownerReferencesLive[0].Name != exporterScraperConfig.Name ||
		ownerReferencesLive[0].UID != exporterScraperConfig.UID ||
		ownerReferencesLive[0].APIVersion != exporterScraperConfig.APIVersion {
		return false
	}

	return true
}

func CheckConfigMap(configMap corev1.ConfigMap, exporterScraperConfig finopsv1.ExporterScraperConfig) bool {
	if configMap.Name != exporterScraperConfig.Name+"-configmap" {
		log.Logger.Info().Msg("name wrong")
		return false
	}

	yamlData, err := yaml.Marshal(exporterScraperConfig)
	if err != nil {
		return false
	}

	if yamlDataFromLive, ok := configMap.BinaryData["config.yaml"]; ok {
		if strings.Replace(string(yamlData), " ", "", -1) != strings.Replace(string(yamlDataFromLive), " ", "", -1) {
			log.Logger.Info().Msg("bytes different")
			return false
		}
	} else {
		log.Logger.Info().Msg("config.yaml not in map")
		return false
	}

	ownerReferencesLive := configMap.OwnerReferences
	if len(ownerReferencesLive) != 1 {
		log.Logger.Info().Msg("owner len not 1")
		return false
	}

	if ownerReferencesLive[0].Kind != exporterScraperConfig.Kind ||
		ownerReferencesLive[0].Name != exporterScraperConfig.Name ||
		ownerReferencesLive[0].UID != exporterScraperConfig.UID ||
		ownerReferencesLive[0].APIVersion != exporterScraperConfig.APIVersion {
		log.Logger.Info().Msg("owner wrong")
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

	if len(deployment.Spec.Template.Spec.Containers) != 1 {
		log.Logger.Debug().Msg("Container not equal to 1")
		return false
	} else {
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
