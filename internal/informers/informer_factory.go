/*
Copyright 2024.

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

package informers

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"

	finopsdatatypes "github.com/krateoplatformops/finops-data-types/api/v1"
	finopsv1 "github.com/krateoplatformops/finops-operator-exporter/api/v1"
	clientHelper "github.com/krateoplatformops/finops-operator-exporter/internal/helpers/kube/client"
	"github.com/krateoplatformops/finops-operator-exporter/internal/helpers/kube/comparators"
	"github.com/krateoplatformops/finops-operator-exporter/internal/utils"
	operatorscraperapi "github.com/krateoplatformops/finops-operator-scraper/api/v1"
	"github.com/krateoplatformops/provider-runtime/pkg/logging"
	appsv1 "k8s.io/api/apps/v1"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

type InformerFactory struct {
	Logger    logging.Logger
	DynClient *dynamic.DynamicClient
}

func (r *InformerFactory) StartInformer(namespace string, gvr schema.GroupVersionResource) {
	inClusterConfig := ctrl.GetConfigOrDie()

	inClusterConfig.APIPath = "/apis"
	inClusterConfig.GroupVersion = &finopsv1.GroupVersion

	clientSet, err := dynamic.NewForConfig(inClusterConfig)
	if err != nil {
		panic(err)
	}

	fac := dynamicinformer.NewFilteredDynamicSharedInformerFactory(clientSet, 0, namespace, nil)
	informer := fac.ForResource(gvr).Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: func(obj interface{}) {
			item := obj.(*unstructured.Unstructured)
			var toReplace string
			switch kind := item.GetKind(); kind {
			case "Deployment":
				toReplace = "-deployment"
			case "ConfigMap":
				toReplace = "-configmap"
			case "Service":
				toReplace = "-service"
			case "ScraperConfig":
				toReplace = "-scraper"
			}

			exporterScraperConfig, err := r.getConfigurationCR(context.TODO(), strings.Replace(item.GetName(), toReplace, "", 1), item.GetNamespace())
			if err != nil {
				r.Logger.Info("could not obtain exporterScraperConfig, everything deleted, stopping", "resource name", item.GetName(), "resource kind", item.GetKind(), "resource namespace", item.GetNamespace())
				return
			}

			if !exporterScraperConfig.DeletionTimestamp.IsZero() {
				r.Logger.Info("resource was deleted, however config is marked for deletion, stopping", "resource name", item.GetKind(), "deletion timestamp", exporterScraperConfig.DeletionTimestamp)
				return
			}
			err = r.createRestoreObject(context.TODO(), exporterScraperConfig, item.GetKind(), true)
			if err != nil {
				r.Logger.Info("unable to create resource again, stopping", "resource name", item.GetKind(), "error", err)
				return
			}
			r.Logger.Info("Created new copy of resource object", "resource kind", item.GetKind(), "resource namespace", item.GetNamespace(), "resource name", item.GetName())
		},
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			item := newObj.(*unstructured.Unstructured)
			found := false
			if ownerReferences := item.GetOwnerReferences(); len(ownerReferences) > 0 {
				for _, ownerReference := range ownerReferences {
					if ownerReference.Kind == "ExporterScraperConfig" {
						found = true
					}
				}
			}
			if !found {
				return
			}

			var toReplace string
			switch kind := item.GetKind(); kind {
			case "Deployment":
				toReplace = "-deployment"
			case "ConfigMap":
				toReplace = "-configmap"
			case "Service":
				toReplace = "-service"
			case "ScraperConfig":
				toReplace = "-scraper"
			}

			exporterScraperConfig, err := r.getConfigurationCR(context.TODO(), strings.Replace(item.GetName(), toReplace, "", 1), item.GetNamespace())
			if err != nil {
				r.Logger.Info("could not obtain exporterScraperConfig, everything deleted, stopping", "resource name", item.GetName(), "resource kind", item.GetKind(), "resource namespace", item.GetNamespace())
				return
			}

			restore := false

			switch kind := item.GetKind(); kind {
			case "Deployment":
				deploymentObj := &appsv1.Deployment{}
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, deploymentObj)
				if err != nil {
					r.Logger.Info("unable to parse unstructured, stopping", "resource kind", item.GetKind(), "error", err)
					return
				}
				if !comparators.CheckDeployment(*deploymentObj, *exporterScraperConfig) {
					restore = true
				}
			case "ConfigMap":
				configMapObj := &corev1.ConfigMap{}
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, configMapObj)
				if err != nil {
					r.Logger.Info("unable to parse unstructured, stopping", "resource kind", item.GetKind(), "error", err)
					return
				}
				if !comparators.CheckConfigMap(*configMapObj, *exporterScraperConfig) {
					restore = true
				}
			case "Service":
				serviceObj := &corev1.Service{}
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, serviceObj)
				if err != nil {
					r.Logger.Info("unable to parse unstructured, stopping", "resource kind", item.GetKind(), "error", err)
					return
				}
				if !comparators.CheckService(*serviceObj, *exporterScraperConfig) {
					restore = true
				}
			case "ScraperConfig":
				scraperObj := &operatorscraperapi.ScraperConfig{}
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, scraperObj)
				if err != nil {
					r.Logger.Info("unable to parse unstructured, stopping", "resource kind", item.GetKind(), "error", err)
					return
				}
				if !comparators.CheckScraper(*scraperObj, *exporterScraperConfig) {
					restore = true
				}
			}

			if restore {
				err = r.createRestoreObject(context.TODO(), exporterScraperConfig, item.GetKind(), false)
				if err != nil {
					r.Logger.Info("unable to create resource again, stopping", "resource kind", item.GetKind(), "error", err)
					return
				}
				r.Logger.Info("Restored resource object", "resource kind", item.GetKind(), "resource namespace", item.GetNamespace(), "resource name", item.GetName())
			}
		},
	})

	go informer.Run(make(<-chan struct{}))
}

func (r *InformerFactory) getConfigurationCR(ctx context.Context, name string, namespace string) (*finopsv1.ExporterScraperConfig, error) {
	configurationName := &finopsdatatypes.ObjectRef{Name: name, Namespace: namespace}

	exporterScraperConfigUnstructured, err := clientHelper.GetObj(ctx, configurationName, "finops.krateo.io/v1", "exporterscraperconfigs", r.DynClient)
	if err != nil {
		return &finopsv1.ExporterScraperConfig{}, fmt.Errorf("unable to fetch finopsv1.ExporterScraperConfig: %v", err)
	}
	exporterScraperConfig := &finopsv1.ExporterScraperConfig{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(exporterScraperConfigUnstructured.Object, exporterScraperConfig)
	if err != nil {
		return &finopsv1.ExporterScraperConfig{}, fmt.Errorf("unable to convert unstructured to finopsv1.ExporterScraperConfig: %v", err)
	}

	return exporterScraperConfig, nil
}

func (r *InformerFactory) createRestoreObject(ctx context.Context, exporterScraperConfig *finopsv1.ExporterScraperConfig, objectToRestoreKind string, create bool) error {
	var err error
	var genericObjectUnstructured *unstructured.Unstructured
	var resource string

	switch objectToRestoreKind {
	case "Deployment":
		genericObject, _ := utils.GetGenericExporterDeployment(exporterScraperConfig)
		genericObjectUnstructured, err = clientHelper.ToUnstructured(genericObject)
		if err != nil {
			return fmt.Errorf("error while converting unstructured deployment to deployment: %v", err)
		}
		resource = "deployments"
	case "ConfigMap":
		genericObject, _ := utils.GetGenericExporterConfigMap(exporterScraperConfig)
		genericObjectUnstructured, err = clientHelper.ToUnstructured(genericObject)
		if err != nil {
			return fmt.Errorf("error while converting unstructured configmap to configmap: %v", err)
		}
		resource = "configmaps"
	case "Service":
		genericObject, _ := utils.GetGenericExporterService(exporterScraperConfig)
		genericObjectUnstructured, err = clientHelper.ToUnstructured(genericObject)
		if err != nil {
			return fmt.Errorf("error while converting unstructured service to service: %v", err)
		}
		resource = "services"
	case "ScraperConfig":
		// Get the service to know on which port it created the service
		genericExporterServiceUnstructuredCreated, err := clientHelper.GetObj(ctx, &finopsdatatypes.ObjectRef{Name: exporterScraperConfig.Name + "-service", Namespace: exporterScraperConfig.Namespace}, "v1", "services", r.DynClient)
		if err != nil {
			return fmt.Errorf("error while getting exporter service: %v", err)
		}
		genericExporterServiceCreated := &corev1.Service{}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(genericExporterServiceUnstructuredCreated.Object, genericExporterServiceCreated)
		if err != nil {
			return fmt.Errorf("error while converting unstructured service to service: %v", err)
		}

		serviceIp := genericExporterServiceCreated.Spec.ClusterIP
		servicePort := -1
		for _, port := range genericExporterServiceCreated.Spec.Ports {
			servicePort = int(port.TargetPort.IntVal)
		}

		genericObject, err := utils.GetGenericExporterScraperConfig(exporterScraperConfig, serviceIp, servicePort)
		if err != nil {
			return fmt.Errorf("error while creating generic exporter scraper config: %v", err)
		}
		genericObjectUnstructured, err = clientHelper.ToUnstructured(genericObject)
		if err != nil {
			return fmt.Errorf("error while converting unstructured scraper config to scraper config: %v", err)
		}
		resource = "scraperconfigs"
	}
	if err != nil {
		return err
	}

	if create {
		err = clientHelper.CreateObj(ctx, genericObjectUnstructured, resource, r.DynClient)
	} else {
		err = clientHelper.UpdateObj(ctx, genericObjectUnstructured, resource, r.DynClient)
	}
	if err != nil {
		return err
	}
	return nil
}
