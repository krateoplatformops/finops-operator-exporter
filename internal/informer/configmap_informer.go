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

package informer

import (
	"bytes"
	"context"
	"strings"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	finopsv1 "operator-exporter/api/v1"
	"operator-exporter/internal/utils"

	corev1 "k8s.io/api/core/v1"
)

type ConfigMapReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=finops.krateo.io,resources=exporterscraperconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=finops.krateo.io,resources=scraperconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=finops.krateo.io,resources=databaseconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=finops.krateo.io,resources=exporterscraperconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=finops.krateo.io,resources=exporterscraperconfigs/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete

func (r *ConfigMapReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.Log.WithValues("FinOps.V1", "CONFIGMAP")
	var err error

	var configMap corev1.ConfigMap
	// ConfigMap does not exist, check if the ExporterScraperConfig exists
	if err = r.Get(ctx, req.NamespacedName, &configMap); err != nil {
		logger.Info("unable to fetch corev1.ConfigMap " + req.Name + " " + req.Namespace)
		exporterScraperConfig, err := r.getConfigurationCR(ctx, strings.Replace(req.Name, "-configmap", "", 1), req.Namespace)
		if err != nil {
			logger.Info("Unable to fetch exporterScraperConfig for " + strings.Replace(req.Name, "-configmap", "", 1) + " " + req.Namespace)
			return ctrl.Result{Requeue: false}, client.IgnoreNotFound(err)
		}
		err = r.createRestoreConfigMapAgain(ctx, exporterScraperConfig, false)
		if err != nil {
			logger.Error(err, "Unable to create ConfigMap again "+req.Name+" "+req.Namespace)
			return ctrl.Result{Requeue: false}, client.IgnoreNotFound(err)
		}
		logger.Info("Created configmap again: " + req.Name + " " + req.Namespace)

	}

	if ownerReferences := configMap.GetOwnerReferences(); len(ownerReferences) > 0 {
		if ownerReferences[0].Kind == "ExporterScraperConfig" {
			logger.Info("Called for " + req.Name + " " + req.Namespace + " owner: " + ownerReferences[0].Kind)
			exporterScraperConfig, err := r.getConfigurationCR(ctx, strings.Replace(req.Name, "-configmap", "", 1), req.Namespace)
			if err != nil {
				return ctrl.Result{}, err
			}
			if !checkConfigMap(configMap, exporterScraperConfig) {
				err = r.createRestoreConfigMapAgain(ctx, exporterScraperConfig, true)
				if err != nil {
					return ctrl.Result{}, err
				}
				logger.Info("Updated configmap: " + req.Name + " " + req.Namespace)
			}
		}
	} else {
		return ctrl.Result{Requeue: false}, nil
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigMapReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		Complete(r)
}

func (r *ConfigMapReconciler) getConfigurationCR(ctx context.Context, name string, namespace string) (finopsv1.ExporterScraperConfig, error) {
	var exporterScraperConfig finopsv1.ExporterScraperConfig
	configurationName := types.NamespacedName{Name: name, Namespace: namespace}
	if err := r.Get(ctx, configurationName, &exporterScraperConfig); err != nil {
		log.Log.Error(err, "unable to fetch finopsv1.ExporterScraperConfig")
		return finopsv1.ExporterScraperConfig{}, err
	}
	return exporterScraperConfig, nil
}

func (r *ConfigMapReconciler) createRestoreConfigMapAgain(ctx context.Context, exporterScraperConfig finopsv1.ExporterScraperConfig, restore bool) error {
	genericExporterConfigMap, err := utils.GetGenericExporterConfigMap(exporterScraperConfig)
	if err != nil {
		return err
	}
	if restore {
		err = r.Update(ctx, genericExporterConfigMap)
	} else {
		err = r.Create(ctx, genericExporterConfigMap)
	}
	if err != nil {
		return err
	}
	return nil
}

func checkConfigMap(configMap corev1.ConfigMap, exporterScraperConfig finopsv1.ExporterScraperConfig) bool {
	if configMap.Name != exporterScraperConfig.Name+"-configmap" {
		return false
	}

	yamlData, err := yaml.Marshal(exporterScraperConfig.Spec.ExporterConfig)
	if err != nil {
		return false
	}

	binaryData := configMap.BinaryData
	if yamlDataFromLive, ok := binaryData["config.yaml"]; ok {
		if !bytes.Equal(yamlData, yamlDataFromLive) {
			return false
		}
	} else {
		return false
	}

	ownerReferencesLive := configMap.OwnerReferences
	if len(ownerReferencesLive) != 1 {
		return false
	}

	if ownerReferencesLive[0].Kind != exporterScraperConfig.Kind ||
		ownerReferencesLive[0].Name != exporterScraperConfig.Name ||
		ownerReferencesLive[0].UID != exporterScraperConfig.UID ||
		ownerReferencesLive[0].APIVersion != exporterScraperConfig.APIVersion {
		return false
	}

	return true
}
