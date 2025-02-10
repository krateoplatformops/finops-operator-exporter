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
	"errors"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	finopsDataTypes "github.com/krateoplatformops/finops-data-types/api/v1"
	finopsv1 "github.com/krateoplatformops/finops-operator-exporter/api/v1"
	prv1 "github.com/krateoplatformops/provider-runtime/apis/common/v1"
	"github.com/krateoplatformops/provider-runtime/pkg/controller"
	"github.com/krateoplatformops/provider-runtime/pkg/event"
	"github.com/krateoplatformops/provider-runtime/pkg/logging"
	"github.com/krateoplatformops/provider-runtime/pkg/ratelimiter"
	"github.com/krateoplatformops/provider-runtime/pkg/reconciler"
	"github.com/krateoplatformops/provider-runtime/pkg/resource"

	clientHelper "github.com/krateoplatformops/finops-operator-exporter/internal/helpers/kube/client"
	"github.com/krateoplatformops/finops-operator-exporter/internal/helpers/kube/comparators"
	utils "github.com/krateoplatformops/finops-operator-exporter/internal/utils"
)

const (
	errNotExporterScraperConfig = "managed resource is not an exporter scraper config custom resource"
)

//+kubebuilder:rbac:groups=finops.krateo.io,namespace=finops,resources=exporterscraperconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=finops.krateo.io,namespace=finops,resources=exporterscraperconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=finops.krateo.io,namespace=finops,resources=exporterscraperconfigs/finalizers,verbs=update
//+kubebuilder:rbac:groups=finops.krateo.io,namespace=finops,resources=scraperconfigs,verbs=get;create;update
//+kubebuilder:rbac:groups=finops.krateo.io,namespace=finops,resources=databaseconfigs,verbs=get;create;update
//+kubebuilder:rbac:groups=apps,namespace=finops,resources=deployments,verbs=get;create;delete;list;update;watch
//+kubebuilder:rbac:groups=core,namespace=finops,resources=configmaps,verbs=get;create;delete;list;update;watch
//+kubebuilder:rbac:groups=core,namespace=finops,resources=services,verbs=get;create;delete;list;update;watch
//+kubebuilder:rbac:groups=core,namespace=finops,resources=secrets,verbs=get;create;delete;list;update;watch

func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := reconciler.ControllerName(finopsv1.GroupKind)

	log := o.Logger.WithValues("controller", name)
	log.Info("controller", "name", name)

	recorder := mgr.GetEventRecorderFor(name)

	r := reconciler.NewReconciler(mgr,
		resource.ManagedKind(finopsv1.GroupVersionKind),
		reconciler.WithExternalConnecter(&connector{
			log:          log,
			recorder:     recorder,
			pollInterval: o.PollInterval,
		}),
		reconciler.WithPollInterval(o.PollInterval),
		reconciler.WithLogger(log),
		reconciler.WithRecorder(event.NewAPIRecorder(recorder)))

	log.Debug("polling rate", "rate", o.PollInterval)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&finopsv1.ExporterScraperConfig{}).
		Complete(ratelimiter.New(name, r, o.GlobalRateLimiter))
}

type connector struct {
	pollInterval time.Duration
	log          logging.Logger
	recorder     record.EventRecorder
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (reconciler.ExternalClient, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve rest.InClusterConfig: %w", err)
	}

	dynClient, err := clientHelper.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to create dynamic client: %w", err)
	}

	return &external{
		cfg:             cfg,
		dynClient:       dynClient,
		sinceLastUpdate: make(map[string]time.Time),
		pollInterval:    c.pollInterval,
		log:             c.log,
		rec:             c.recorder,
	}, nil
}

type external struct {
	cfg             *rest.Config
	dynClient       *dynamic.DynamicClient
	sinceLastUpdate map[string]time.Time
	pollInterval    time.Duration
	log             logging.Logger
	rec             record.EventRecorder
}

func (c *external) Disconnect(_ context.Context) error {
	return nil // NOOP
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (reconciler.ExternalObservation, error) {
	exporterScraperConfig, ok := mg.(*finopsv1.ExporterScraperConfig)
	if !ok {
		return reconciler.ExternalObservation{}, errors.New(errNotExporterScraperConfig)
	}

	// Check if a deployment for this configuration already exists
	existingObjDeployment, err := clientHelper.GetObj(ctx,
		&finopsDataTypes.ObjectRef{
			Name:      exporterScraperConfig.Status.ActiveExporter.Name,
			Namespace: exporterScraperConfig.Status.ActiveExporter.Namespace},
		"apps/v1", "deployments", e.dynClient)
	if err != nil {
		e.rec.Eventf(exporterScraperConfig, corev1.EventTypeWarning, "could not get exporterscraperconfig deployment", "object name: %s", exporterScraperConfig.Status.ActiveExporter.Name)
		return reconciler.ExternalObservation{
			ResourceExists: false,
		}, nil
	}

	// ConfigMap status objRef and pointer for GET
	existingObjConfigMap, err := clientHelper.GetObj(ctx,
		&finopsDataTypes.ObjectRef{
			Name:      exporterScraperConfig.Status.ConfigMap.Name,
			Namespace: exporterScraperConfig.Status.ConfigMap.Namespace},
		"v1", "configmaps", e.dynClient)
	if err != nil {
		e.rec.Eventf(exporterScraperConfig, corev1.EventTypeWarning, "could not get exporterscraperconfig configmap", "object name: %s", exporterScraperConfig.Status.ConfigMap.Name)
		return reconciler.ExternalObservation{
			ResourceExists: false,
		}, nil
	}

	// Service status objRef and pointer for GET
	existingObjService, err := clientHelper.GetObj(ctx,
		&finopsDataTypes.ObjectRef{
			Name:      exporterScraperConfig.Status.Service.Name,
			Namespace: exporterScraperConfig.Status.Service.Namespace},
		"v1", "services", e.dynClient)
	if err != nil {
		e.rec.Eventf(exporterScraperConfig, corev1.EventTypeWarning, "could not get exporterscraperconfig service", "object name: %s", exporterScraperConfig.Status.Service.Name)
		return reconciler.ExternalObservation{
			ResourceExists: false,
		}, nil
	}

	// Check if all elements of the deployment exist
	// If any the objects does not exist, something happend, reconcile spec-status
	if existingObjDeployment.GetName() == "" || existingObjConfigMap.GetName() == "" || existingObjService.GetName() == "" {
		return reconciler.ExternalObservation{
			ResourceExists: false,
		}, nil
	} else {
		status, notUpToDate, err := checkExporterStatus(ctx, exporterScraperConfig, e.dynClient)
		if err != nil {
			return reconciler.ExternalObservation{
				ResourceExists: false,
			}, fmt.Errorf("could not check exporter status: %v", err)
		}
		exporterScraperConfig.SetConditions(prv1.Available())
		if !status {
			e.rec.Eventf(exporterScraperConfig, corev1.EventTypeWarning, "exporter resources not up-to-date", "object name: %s - first not up-to-date: %s", exporterScraperConfig.Name, notUpToDate)
			return reconciler.ExternalObservation{
				ResourceExists:   true,
				ResourceUpToDate: false,
			}, nil
		}
		return reconciler.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: true,
		}, nil
	}
}

func (e *external) Create(ctx context.Context, mg resource.Managed) error {
	exporterScraperConfig, ok := mg.(*finopsv1.ExporterScraperConfig)
	if !ok {
		return errors.New(errNotExporterScraperConfig)
	}

	exporterScraperConfig.SetConditions(prv1.Creating())

	err := createExporterFromScratch(ctx, exporterScraperConfig, e.dynClient)
	if err != nil {
		return err
	}

	e.rec.Eventf(exporterScraperConfig, corev1.EventTypeNormal, "Completed create", "object name: %s", exporterScraperConfig.Name)

	return nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) error {
	exporterScraperConfig, ok := mg.(*finopsv1.ExporterScraperConfig)
	if !ok {
		return errors.New(errNotExporterScraperConfig)
	}

	genericExporterConfigMap, err := utils.GetGenericExporterConfigMap(exporterScraperConfig)
	if err != nil {
		return err
	}
	genericExporterConfigMapUnstructured, err := clientHelper.ToUnstructured(genericExporterConfigMap)
	if err != nil {
		return err
	}

	genericExporterDeployment, _ := utils.GetGenericExporterDeployment(exporterScraperConfig)
	genericExporterDeploymentUnstructured, err := clientHelper.ToUnstructured(genericExporterDeployment)
	if err != nil {
		return err
	}

	genericExporterService, _ := utils.GetGenericExporterService(exporterScraperConfig)
	genericExporterServiceUnstructured, err := clientHelper.ToUnstructured(genericExporterService)
	if err != nil {
		return err
	}

	err = clientHelper.UpdateObj(ctx, genericExporterConfigMapUnstructured, "configmaps", e.dynClient)
	if err != nil {
		return err
	}

	err = clientHelper.UpdateObj(ctx, genericExporterDeploymentUnstructured, "deployments", e.dynClient)
	if err != nil {
		return err
	}

	err = clientHelper.UpdateObj(ctx, genericExporterServiceUnstructured, "services", e.dynClient)
	if err != nil {
		return err
	}

	e.rec.Eventf(exporterScraperConfig, corev1.EventTypeNormal, "Completed update", "object name: %s", exporterScraperConfig.Name)
	e.sinceLastUpdate[exporterScraperConfig.Name+exporterScraperConfig.Namespace] = time.Now()

	return nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) error {
	exporterScraperConfig, ok := mg.(*finopsv1.ExporterScraperConfig)
	if !ok {
		return errors.New(errNotExporterScraperConfig)
	}

	exporterScraperConfig.SetConditions(prv1.Deleting())

	err := clientHelper.DeleteObj(ctx, &finopsDataTypes.ObjectRef{Name: exporterScraperConfig.Name + "-deployment", Namespace: exporterScraperConfig.Namespace}, "apps/v1", "deployments", e.dynClient)
	if err != nil {
		return fmt.Errorf("error while deleting Deployment %v", err)
	}

	err = clientHelper.DeleteObj(ctx, &finopsDataTypes.ObjectRef{Name: exporterScraperConfig.Name + "-configmap", Namespace: exporterScraperConfig.Namespace}, "v1", "configmaps", e.dynClient)
	if err != nil {
		return fmt.Errorf("error while deleting ConfigMap %v", err)
	}

	err = clientHelper.DeleteObj(ctx, &finopsDataTypes.ObjectRef{Name: exporterScraperConfig.Name + "-service", Namespace: exporterScraperConfig.Namespace}, "v1", "services", e.dynClient)
	if err != nil {
		return fmt.Errorf("error while deleting Service %v", err)
	}

	e.rec.Eventf(exporterScraperConfig, corev1.EventTypeNormal, "Received delete event", "removed deployment, configmap and service objects")
	return nil
}

func createExporterFromScratch(ctx context.Context, exporterScraperConfig *finopsv1.ExporterScraperConfig, dynClient *dynamic.DynamicClient) error {
	var err error
	// Create the ConfigMap
	genericExporterConfigMap, err := utils.GetGenericExporterConfigMap(exporterScraperConfig)
	if err != nil {
		return err
	}
	genericExporterConfigMapUnstructured, err := clientHelper.ToUnstructured(genericExporterConfigMap)
	if err != nil {
		return err
	}
	if exporterScraperConfig.Status.ConfigMap.Name == "" {
		err = clientHelper.CreateObj(ctx, genericExporterConfigMapUnstructured, "configmaps", dynClient)
		if err != nil {
			return fmt.Errorf("error while creating configmap: %v", err)
		}
		// Update status
		exporterScraperConfig.Status.ConfigMap = corev1.ObjectReference{
			Kind:      genericExporterConfigMap.Kind,
			Namespace: genericExporterConfigMap.Namespace,
			Name:      genericExporterConfigMap.Name,
		}
		exporterScraperConfigUnstructured, err := clientHelper.ToUnstructured(exporterScraperConfig)
		if err != nil {
			return fmt.Errorf("error while converting exporterscraperconfigs to unstructured: %v", err)
		}
		err = clientHelper.UpdateStatus(ctx, exporterScraperConfigUnstructured, "exporterscraperconfigs", dynClient)
		if err != nil {
			return fmt.Errorf("error while updating status for exporterscraperconfig: %v", err)
		}
		exporterScraperConfigUnstructured, err = clientHelper.GetObj(ctx, &finopsDataTypes.ObjectRef{Name: exporterScraperConfig.Name, Namespace: exporterScraperConfig.Namespace}, exporterScraperConfig.APIVersion, "exporterscraperconfigs", dynClient)
		if err != nil {
			return fmt.Errorf("error while getting updated exporterscraperconfig (status): %v", err)
		}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(exporterScraperConfigUnstructured.Object, exporterScraperConfig)
		if err != nil {
			return fmt.Errorf("error while converting unstructured service to service: %v", err)
		}
	}

	// Create the generic exporter deployment
	genericExporterDeployment, _ := utils.GetGenericExporterDeployment(exporterScraperConfig)
	genericExporterDeploymentUnstructured, err := clientHelper.ToUnstructured(genericExporterDeployment)
	if err != nil {
		return err
	}
	if exporterScraperConfig.Status.ActiveExporter.Name == "" {
		err = clientHelper.CreateObj(ctx, genericExporterDeploymentUnstructured, "deployments", dynClient)
		if err != nil {
			return fmt.Errorf("error while creating deployment: %v", err)
		}
		// Update status
		exporterScraperConfig.Status.ActiveExporter = corev1.ObjectReference{
			Kind:      genericExporterDeployment.Kind,
			Namespace: genericExporterDeployment.Namespace,
			Name:      genericExporterDeployment.Name,
		}
		exporterScraperConfigUnstructured, err := clientHelper.ToUnstructured(exporterScraperConfig)
		if err != nil {
			return fmt.Errorf("error while converting exporterscraperconfigs to unstructured: %v", err)
		}
		err = clientHelper.UpdateStatus(ctx, exporterScraperConfigUnstructured, "exporterscraperconfigs", dynClient)
		if err != nil {
			return fmt.Errorf("error while updating status for exporterscraperconfig: %v", err)
		}
		exporterScraperConfigUnstructured, err = clientHelper.GetObj(ctx, &finopsDataTypes.ObjectRef{Name: exporterScraperConfig.Name, Namespace: exporterScraperConfig.Namespace}, exporterScraperConfig.APIVersion, "exporterscraperconfigs", dynClient)
		if err != nil {
			return fmt.Errorf("error while getting updated exporterscraperconfig (status): %v", err)
		}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(exporterScraperConfigUnstructured.Object, exporterScraperConfig)
		if err != nil {
			return fmt.Errorf("error while converting unstructured service to service: %v", err)
		}
	}

	// Create the Service
	genericExporterService, _ := utils.GetGenericExporterService(exporterScraperConfig)
	genericExporterServiceUnstructured, err := clientHelper.ToUnstructured(genericExporterService)
	if err != nil {
		return err
	}
	if exporterScraperConfig.Status.Service.Name == "" {
		err = clientHelper.CreateObj(ctx, genericExporterServiceUnstructured, "services", dynClient)
		if err != nil {
			return fmt.Errorf("error while creating service: %v", err)
		}
		// Update status
		exporterScraperConfig.Status.Service = corev1.ObjectReference{
			Kind:      genericExporterService.Kind,
			Namespace: genericExporterService.Namespace,
			Name:      genericExporterService.Name,
		}
		exporterScraperConfigUnstructured, err := clientHelper.ToUnstructured(exporterScraperConfig)
		if err != nil {
			return fmt.Errorf("error while converting exporterscraperconfigs to unstructured: %v", err)
		}
		err = clientHelper.UpdateStatus(ctx, exporterScraperConfigUnstructured, "exporterscraperconfigs", dynClient)
		if err != nil {
			return fmt.Errorf("error while updating status for exporterscraperconfig: %v", err)
		}
	}

	// Get the service to know on which port it created the service
	genericExporterServiceUnstructuredCreated, err := clientHelper.GetObj(ctx, &finopsDataTypes.ObjectRef{Name: genericExporterService.Name, Namespace: genericExporterService.Namespace}, "v1", "services", dynClient)
	if err != nil {
		return fmt.Errorf("error while getting just created service: %v", err)
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

	// Create the CR to start the Scraper Operator
	err = utils.CreateScraperCR(ctx, *exporterScraperConfig, serviceIp, servicePort)
	if err != nil {
		return fmt.Errorf("error while creating the scraper cr: %v", err)
	}

	return nil
}

func checkExporterStatus(ctx context.Context, exporterScraperConfig *finopsv1.ExporterScraperConfig, dynClient *dynamic.DynamicClient) (bool, string, error) {
	// Check if a deployment for this configuration already exists
	existingObjDeploymentUnstructured, err := clientHelper.GetObj(ctx,
		&finopsDataTypes.ObjectRef{
			Name:      exporterScraperConfig.Status.ActiveExporter.Name,
			Namespace: exporterScraperConfig.Status.ActiveExporter.Namespace},
		"apps/v1", "deployments", dynClient)
	if err != nil {
		return false, "", fmt.Errorf("could not obtain exporter deployment: %v", err)
	}
	existingObjDeployment := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(existingObjDeploymentUnstructured.Object, existingObjDeployment)
	if err != nil {
		return false, "", err
	}
	if !comparators.CheckDeployment(*existingObjDeployment, *exporterScraperConfig) {
		return false, "Deployment", nil
	}

	// ConfigMap status objRef
	existingObjConfigMapUnstructured, err := clientHelper.GetObj(ctx,
		&finopsDataTypes.ObjectRef{
			Name:      exporterScraperConfig.Status.ConfigMap.Name,
			Namespace: exporterScraperConfig.Status.ConfigMap.Namespace},
		"v1", "configmaps", dynClient)
	if err != nil {
		return false, "", fmt.Errorf("could not obtain exporter configmap: %v", err)
	}
	existingObjConfigMap := &corev1.ConfigMap{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(existingObjConfigMapUnstructured.Object, existingObjConfigMap)
	if err != nil {
		return false, "", err
	}
	if !comparators.CheckConfigMap(*existingObjConfigMap, *exporterScraperConfig) {
		return false, "ConfigMap", nil
	}

	// Service status objRef
	existingObjServiceUnstructured, err := clientHelper.GetObj(ctx,
		&finopsDataTypes.ObjectRef{
			Name:      exporterScraperConfig.Status.Service.Name,
			Namespace: exporterScraperConfig.Status.Service.Namespace},
		"v1", "services", dynClient)
	if err != nil {
		return false, "", fmt.Errorf("could not obtain exporter service: %v", err)
	}
	existingObjService := &corev1.Service{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(existingObjServiceUnstructured.Object, existingObjService)
	if err != nil {
		return false, "", err
	}
	if !comparators.CheckService(*existingObjService, *exporterScraperConfig) {
		return false, "Service", nil
	}

	return true, "", nil
}
