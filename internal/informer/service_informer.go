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
	"context"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	finopsv1 "github.com/krateoplatformops/finops-operator-exporter/api/v1"
	"github.com/krateoplatformops/finops-operator-exporter/internal/utils"

	corev1 "k8s.io/api/core/v1"
)

type ServiceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.Log.WithValues("FinOps.V1", "SERVICE")
	var err error

	var service corev1.Service
	// Service does not exist, check if the ExporterScraperConfig exists
	if err = r.Get(ctx, req.NamespacedName, &service); err != nil {
		logger.Info("unable to fetch corev1.Service " + req.Name + " " + req.Namespace)
		exporterScraperConfig, err := r.getConfigurationCR(ctx, strings.Replace(req.Name, "-service", "", 1), req.Namespace)
		if err != nil {
			logger.Info("Unable to fetch exporterScraperConfig for " + strings.Replace(req.Name, "-service", "", 1) + " " + req.Namespace)
			return ctrl.Result{Requeue: false}, client.IgnoreNotFound(err)
		}
		err = r.createRestoreServiceAgain(ctx, exporterScraperConfig, false)
		if err != nil {
			logger.Error(err, "Unable to create Service again "+req.Name+" "+req.Namespace)
			return ctrl.Result{Requeue: false}, client.IgnoreNotFound(err)
		}
		logger.Info("Created service again: " + req.Name + " " + req.Namespace)

	}

	if ownerReferences := service.GetOwnerReferences(); len(ownerReferences) > 0 {
		if ownerReferences[0].Kind == "ExporterScraperConfig" {
			logger.Info("Called for " + req.Name + " " + req.Namespace + " owner: " + ownerReferences[0].Kind)
			exporterScraperConfig, err := r.getConfigurationCR(ctx, strings.Replace(req.Name, "-service", "", 1), req.Namespace)
			if err != nil {
				return ctrl.Result{}, err
			}
			if !checkService(service, exporterScraperConfig) {
				err = r.createRestoreServiceAgain(ctx, exporterScraperConfig, true)
				if err != nil {
					return ctrl.Result{}, err
				}
				logger.Info("Updated service: " + req.Name + " " + req.Namespace)
			}
		}
	} else {
		return ctrl.Result{Requeue: false}, nil
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		Complete(r)
}

func (r *ServiceReconciler) getConfigurationCR(ctx context.Context, name string, namespace string) (finopsv1.ExporterScraperConfig, error) {
	var exporterScraperConfig finopsv1.ExporterScraperConfig
	configurationName := types.NamespacedName{Name: name, Namespace: namespace}
	if err := r.Get(ctx, configurationName, &exporterScraperConfig); err != nil {
		log.Log.Error(err, "unable to fetch finopsv1.ExporterScraperConfig")
		return finopsv1.ExporterScraperConfig{}, err
	}
	return exporterScraperConfig, nil
}

func (r *ServiceReconciler) createRestoreServiceAgain(ctx context.Context, exporterScraperConfig finopsv1.ExporterScraperConfig, restore bool) error {
	genericExporterService, err := utils.GetGenericExporterService(exporterScraperConfig)
	if err != nil {
		return err
	}
	if restore {
		err = r.Update(ctx, genericExporterService)
	} else {
		err = r.Create(ctx, genericExporterService)
	}
	if err != nil {
		return err
	}
	return nil
}

func checkService(service corev1.Service, exporterScraperConfig finopsv1.ExporterScraperConfig) bool {
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
