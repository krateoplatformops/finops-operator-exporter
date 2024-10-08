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

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	finopsv1 "github.com/krateoplatformops/finops-operator-exporter/api/v1"

	utils "github.com/krateoplatformops/finops-operator-exporter/internal/utils"
)

// ExporterScraperConfigReconciler reconciles a ExporterScraperConfig object
type ExporterScraperConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=finops.krateo.io,namespace=finops,resources=exporterscraperconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=finops.krateo.io,namespace=finops,resources=exporterscraperconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=finops.krateo.io,namespace=finops,resources=exporterscraperconfigs/finalizers,verbs=update
//+kubebuilder:rbac:groups=finops.krateo.io,namespace=finops,resources=scraperconfigs,verbs=get;create;update
//+kubebuilder:rbac:groups=finops.krateo.io,namespace=finops,resources=databaseconfigs,verbs=get;create;update
//+kubebuilder:rbac:groups=apps,namespace=finops,resources=deployments,verbs=get;create;list;update;watch
//+kubebuilder:rbac:groups=core,namespace=finops,resources=configmaps,verbs=get;create;list;update
//+kubebuilder:rbac:groups=core,namespace=finops,resources=services,verbs=get;create;list;update;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ExporterScraperConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.Log.WithValues("FinOps.V1", req.NamespacedName)
	var err error
	// Get the request object
	var exporterScraperConfig finopsv1.ExporterScraperConfig
	if err := r.Get(ctx, req.NamespacedName, &exporterScraperConfig); err != nil {
		logger.Error(err, "unable to fetch finopsv1.ExporterScraperConfig")
		return ctrl.Result{Requeue: false}, client.IgnoreNotFound(err)
	}

	if exporterScraperConfig.Spec.ExporterConfig.MetricType == "" {
		exporterScraperConfig.Spec.ExporterConfig.MetricType = "cost"
		r.Update(ctx, &exporterScraperConfig)
		return ctrl.Result{Requeue: true}, nil
	}

	// Check if a deployment for this configuration already exists
	existingObjDeployment := &appsv1.Deployment{}
	ExistingDeploymentNamespace := exporterScraperConfig.Status.ActiveExporter.Namespace
	ExistingDeploymentName := exporterScraperConfig.Status.ActiveExporter.Name

	// ConfigMap status objRef and pointer for GET
	ExistingConfigMapNamespace := exporterScraperConfig.Status.ConfigMap.Namespace
	ExistingConfigMapName := exporterScraperConfig.Status.ConfigMap.Name
	existingObjConfigMap := &corev1.ConfigMap{}
	// Service status objRef and pointer for GET
	ExistingServiceNamespace := exporterScraperConfig.Status.Service.Namespace
	ExistingServiceMapName := exporterScraperConfig.Status.Service.Name
	existingObjService := &corev1.Service{}

	// Check if all elements of the deployment exist

	_ = r.Get(ctx, types.NamespacedName{Namespace: ExistingDeploymentNamespace, Name: ExistingDeploymentName}, existingObjDeployment)
	_ = r.Get(ctx, types.NamespacedName{Namespace: ExistingConfigMapNamespace, Name: ExistingConfigMapName}, existingObjConfigMap)
	_ = r.Get(ctx, types.NamespacedName{Namespace: ExistingServiceNamespace, Name: ExistingServiceMapName}, existingObjService)
	// If any the objects does not exist, something happend, reconcile spec-status
	if existingObjDeployment.Name == "" || existingObjConfigMap.Name == "" || existingObjService.Name == "" {
		if err = r.createExporterFromScratch(ctx, req, exporterScraperConfig); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	} else if err = r.checkExporterStatus(ctx, exporterScraperConfig); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ExporterScraperConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&finopsv1.ExporterScraperConfig{}).
		Complete(r)
}

func (r *ExporterScraperConfigReconciler) createExporterFromScratch(ctx context.Context, req ctrl.Request, exporterScraperConfig finopsv1.ExporterScraperConfig) error {

	var err error
	// Create the ConfigMap first
	// Check if the ConfigMap exists
	genericExporterConfigMap := &corev1.ConfigMap{}
	_ = r.Get(context.Background(), types.NamespacedName{
		Namespace: req.Namespace,
		Name:      exporterScraperConfig.Name + "-configmap",
	}, genericExporterConfigMap)
	// If it does not exist, create it
	if genericExporterConfigMap.ObjectMeta.Name == "" {
		genericExporterConfigMap, err = utils.GetGenericExporterConfigMap(exporterScraperConfig)
		if err != nil {
			return err
		}
		err = r.Create(ctx, genericExporterConfigMap)
		if err != nil {
			return err
		}
	}

	// Create the generic exporter deployment
	genericExporterDeployment := &appsv1.Deployment{}
	_ = r.Get(context.Background(), types.NamespacedName{
		Namespace: req.Namespace,
		Name:      exporterScraperConfig.Name + "-deployment",
	}, genericExporterDeployment)
	if genericExporterDeployment.ObjectMeta.Name == "" {
		genericExporterDeployment, err = utils.GetGenericExporterDeployment(exporterScraperConfig)
		if err != nil {
			return err
		}
		// Create the actual deployment
		err = r.Create(ctx, genericExporterDeployment)
		if err != nil {
			return err
		}
	}

	// Create the Service
	// Check if the Service exists
	genericExporterService := &corev1.Service{}
	_ = r.Get(context.Background(), types.NamespacedName{
		Namespace: req.Namespace,
		Name:      exporterScraperConfig.Name + "-service",
	}, genericExporterService)
	// If it does not exist, create it
	if genericExporterService.ObjectMeta.Name == "" {
		genericExporterService, _ = utils.GetGenericExporterService(exporterScraperConfig)
		err = r.Create(ctx, genericExporterService)
		if err != nil {
			return err
		}
	}

	exporterScraperConfig.Status.ActiveExporter = corev1.ObjectReference{
		Kind:      genericExporterDeployment.Kind,
		Namespace: genericExporterDeployment.Namespace,
		Name:      genericExporterDeployment.Name,
	}
	exporterScraperConfig.Status.ConfigMap = corev1.ObjectReference{
		Kind:      genericExporterConfigMap.Kind,
		Namespace: genericExporterConfigMap.Namespace,
		Name:      genericExporterConfigMap.Name,
	}
	exporterScraperConfig.Status.Service = corev1.ObjectReference{
		Kind:      genericExporterService.Kind,
		Namespace: genericExporterService.Namespace,
		Name:      genericExporterService.Name,
	}

	serviceIp := genericExporterService.Spec.ClusterIP
	servicePort := -1
	for _, port := range genericExporterService.Spec.Ports {
		servicePort = int(port.TargetPort.IntVal)
	}

	// Create the CR to start the Scraper Operator
	err = utils.CreateScraperCR(ctx, exporterScraperConfig, serviceIp, servicePort)
	if err != nil {
		return err
	}
	return nil
}

func (r *ExporterScraperConfigReconciler) checkExporterStatus(ctx context.Context, exporterScraperConfig finopsv1.ExporterScraperConfig) error {
	genericExporterConfigMap, err := utils.GetGenericExporterConfigMap(exporterScraperConfig)
	if err != nil {
		return err
	}
	genericExporterDeployment, _ := utils.GetGenericExporterDeployment(exporterScraperConfig)
	genericExporterService, _ := utils.GetGenericExporterService(exporterScraperConfig)

	err = r.Update(ctx, genericExporterConfigMap)
	if err != nil {
		return err
	}

	err = r.Update(ctx, genericExporterDeployment)
	if err != nil {
		return err
	}

	err = r.Update(ctx, genericExporterService)
	if err != nil {
		return err
	}

	return nil
}
