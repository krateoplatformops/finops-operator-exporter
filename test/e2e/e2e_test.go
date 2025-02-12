package test

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	operatorlogger "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/support/kind"

	operatorexporterapi "github.com/krateoplatformops/finops-operator-exporter/api/v1"
	operatorexporter "github.com/krateoplatformops/finops-operator-exporter/internal/controller"
	"github.com/krateoplatformops/finops-operator-exporter/internal/helpers/kube/comparators"
	"github.com/krateoplatformops/finops-operator-exporter/internal/utils"
	"github.com/krateoplatformops/provider-runtime/pkg/controller"
	"github.com/krateoplatformops/provider-runtime/pkg/logging"
	"github.com/krateoplatformops/provider-runtime/pkg/ratelimiter"
	"github.com/rs/zerolog/log"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	clientHelper "github.com/krateoplatformops/finops-operator-exporter/internal/helpers/kube/client"
	informer_factory "github.com/krateoplatformops/finops-operator-exporter/internal/informers"
)

type contextKey string

var (
	testenv env.Environment
	scheme  = runtime.NewScheme()
)

const (
	testNamespace   = "finops-test" // If you changed this test environment, you need to change the RoleBinding in the "deploymentsPath" folder
	crdsPath1       = "../../config/crd/bases"
	crdsPath2       = "./manifests/crds"
	deploymentsPath = "./manifests/deployments"
	toTest          = "./manifests/to_test/"
	testName        = "exporterscraperconfig-sample"
)

func init() {
	// Add the required schemes
	operatorexporterapi.AddToScheme(scheme)
	appsv1.AddToScheme(scheme)
	corev1.AddToScheme(scheme)
}

func TestMain(m *testing.M) {
	testenv = env.New()
	kindClusterName := "krateo-test"

	testenv.Setup(
		envfuncs.CreateCluster(kind.NewProvider(), kindClusterName),
		envfuncs.CreateNamespace(testNamespace),
		envfuncs.SetupCRDs(crdsPath1, "*"),
		envfuncs.SetupCRDs(crdsPath2, "*"),
	)

	testenv.Finish(
		envfuncs.DeleteNamespace(testNamespace),
		envfuncs.TeardownCRDs(crdsPath1, "*"),
		envfuncs.TeardownCRDs(crdsPath2, "*"),
		envfuncs.DestroyCluster(kindClusterName),
	)

	os.Exit(testenv.Run(m))
}

func TestExporter(t *testing.T) {
	create := features.New("Create").
		WithLabel("type", "CR and resources").
		Setup(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			r, err := resources.New(c.Client().RESTConfig())
			if err != nil {
				t.Fatal(err)
			}

			// Start the controller manager
			err = startTestManager(ctx)
			if err != nil {
				t.Fatal(err)
			}

			ctx = context.WithValue(ctx, contextKey("client"), r)

			operatorexporterapi.AddToScheme(r.GetScheme())
			r.WithNamespace(testNamespace)

			err = decoder.DecodeEachFile(
				ctx, os.DirFS(deploymentsPath), "*",
				decoder.CreateHandler(r),
				decoder.MutateNamespace(testNamespace),
			)
			if err != nil {
				t.Fatalf("Failed due to error: %s", err)
			}

			// Create test resources
			err = decoder.DecodeEachFile(
				ctx, os.DirFS(toTest), "*",
				decoder.CreateHandler(r),
				decoder.MutateNamespace(testNamespace),
			)
			if err != nil {
				t.Fatal(err)
			}

			// Wait for controller to be ready
			time.Sleep(5 * time.Second)

			return ctx
		}).
		Assess("CR", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			r := ctx.Value(contextKey("client")).(*resources.Resources)

			crGet := &operatorexporterapi.ExporterScraperConfig{}
			err := r.Get(ctx, testName, testNamespace, crGet)
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Assess("Resources", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			r := ctx.Value(contextKey("client")).(*resources.Resources)

			deployment := &appsv1.Deployment{}
			configmap := &corev1.ConfigMap{}
			service := &corev1.Service{}

			err := r.Get(ctx, testName+"-deployment", testNamespace, deployment)
			if err != nil {
				t.Fatal(err)
			}

			err = r.Get(ctx, testName+"-configmap", testNamespace, configmap)
			if err != nil {
				t.Fatal(err)
			}

			err = r.Get(ctx, testName+"-service", testNamespace, service)
			if err != nil {
				t.Fatal(err)
			}

			crGet := &operatorexporterapi.ExporterScraperConfig{}
			err = r.Get(ctx, testName, testNamespace, crGet)
			if err != nil {
				t.Fatal(err)
			}

			if crGet.Status.ActiveExporter.Name == "" || crGet.Status.ConfigMap.Name == "" || crGet.Status.Service.Name == "" {
				t.Fatal(fmt.Errorf("missing status update"))
			}
			return ctx
		}).
		Assess("ConfigCorrect", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			r := ctx.Value(contextKey("client")).(*resources.Resources)

			if err := wait.For(
				conditions.New(r).DeploymentAvailable(testName+"-deployment", testNamespace),
				wait.WithTimeout(120*time.Second),
				wait.WithInterval(5*time.Second),
			); err != nil {
				t.Fatal(fmt.Errorf("timed out while waiting for %s-deployment: %w", testName, err))
			}

			time.Sleep(20 * time.Second) // Give the container time to finish startup and error out

			pods := &corev1.PodList{}
			r.List(ctx, pods, resources.WithLabelSelector("scraper="+testName))
			if len(pods.Items) > 1 {
				t.Fatal(fmt.Errorf("number of pods created for config more than 1: %d", len(pods.Items)))
			}

			for _, condition := range pods.Items[0].Status.Conditions {
				if condition.Type == corev1.PodReady {
					if condition.Status != corev1.ConditionTrue {
						t.Fatal(fmt.Errorf("pod %s is not ready: %s %s", pods.Items[0].Name, condition.Type, condition.Status))
					} else {
						for _, container := range pods.Items[0].Status.ContainerStatuses {
							if container.Name == "scraper" {
								if container.RestartCount > 1 {
									t.Fatal(fmt.Errorf("pod %s has restarts: %d", pods.Items[0].Name, container.RestartCount))
								}
							}
						}
					}
				}
			}

			return ctx
		}).Feature()

	delete := features.New("Delete").
		WithLabel("type", "Resources").
		Setup(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			return ctx
		}).
		Assess("Deployment", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			r := ctx.Value(contextKey("client")).(*resources.Resources)

			resource := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testName + "-deployment",
					Namespace: testNamespace,
				},
			}
			err := r.Delete(ctx, resource)
			if err != nil {
				t.Fatal(err)
			}

			time.Sleep(5 * time.Second) // Wait for informer to re-create

			err = r.Get(ctx, testName+"-deployment", testNamespace, resource)
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Assess("ConfigMap", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			r := ctx.Value(contextKey("client")).(*resources.Resources)

			resource := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testName + "-configmap",
					Namespace: testNamespace,
				},
			}
			err := r.Delete(ctx, resource)
			if err != nil {
				t.Fatal(err)
			}

			time.Sleep(5 * time.Second) // Wait for informer to re-create

			err = r.Get(ctx, testName+"-configmap", testNamespace, resource)
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Assess("Service", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			r := ctx.Value(contextKey("client")).(*resources.Resources)

			resource := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testName + "-service",
					Namespace: testNamespace,
				},
			}
			err := r.Delete(ctx, resource)
			if err != nil {
				t.Fatal(err)
			}

			time.Sleep(5 * time.Second) // Wait for informer to re-create

			err = r.Get(ctx, testName+"-service", testNamespace, resource)
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).Feature()

	modify := features.New("Modify").
		WithLabel("type", "Resources").
		Setup(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			return ctx
		}).
		Assess("Deployment", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			r := ctx.Value(contextKey("client")).(*resources.Resources)

			exporterScraperConfig := &operatorexporterapi.ExporterScraperConfig{}
			err := r.Get(ctx, testName, testNamespace, exporterScraperConfig)
			if err != nil {
				t.Fatal(err)
			}
			// This is necessary because it does not get compiled automatically by the GET
			exporterScraperConfig.TypeMeta.Kind = "ExporterScraperConfig"
			exporterScraperConfig.TypeMeta.APIVersion = "finops.krateo.io/v1"

			resource := &appsv1.Deployment{}
			err = r.Get(ctx, testName+"-deployment", testNamespace, resource)
			if err != nil {
				t.Fatal(err)
			}

			resource.Spec.Replicas = utils.Int32Ptr(2)

			err = r.Update(ctx, resource)
			if err != nil {
				t.Fatal(err)
			}

			time.Sleep(5 * time.Second) // Wait for informer to restore

			resource = &appsv1.Deployment{}
			err = r.Get(ctx, testName+"-deployment", testNamespace, resource)
			if err != nil {
				t.Fatal(err)
			}

			if !comparators.CheckDeployment(*resource, *exporterScraperConfig) {
				t.Fatal("Deployment not restored by informer")
			}
			return ctx
		}).
		Assess("ConfigMap", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			r := ctx.Value(contextKey("client")).(*resources.Resources)

			exporterScraperConfig := &operatorexporterapi.ExporterScraperConfig{}
			err := r.Get(ctx, testName, testNamespace, exporterScraperConfig)
			if err != nil {
				t.Fatal(err)
			}
			// This is necessary because it does not get compiled automatically by the GET
			exporterScraperConfig.TypeMeta.Kind = "ExporterScraperConfig"
			exporterScraperConfig.TypeMeta.APIVersion = "finops.krateo.io/v1"

			resource := &corev1.ConfigMap{}
			err = r.Get(ctx, testName+"-configmap", testNamespace, resource)
			if err != nil {
				t.Fatal(err)
			}

			resource.BinaryData["config.yaml"] = []byte("broken")

			err = r.Update(ctx, resource)
			if err != nil {
				t.Fatal(err)
			}

			time.Sleep(5 * time.Second) // Wait for informer to restore

			resource = &corev1.ConfigMap{}
			err = r.Get(ctx, testName+"-configmap", testNamespace, resource)
			if err != nil {
				t.Fatal(err)
			}

			if !comparators.CheckConfigMap(*resource, *exporterScraperConfig) {
				t.Fatal("ConfigMap not restored by informer")
			}
			return ctx
		}).
		Assess("Service", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			r := ctx.Value(contextKey("client")).(*resources.Resources)

			exporterScraperConfig := &operatorexporterapi.ExporterScraperConfig{}
			err := r.Get(ctx, testName, testNamespace, exporterScraperConfig)
			if err != nil {
				t.Fatal(err)
			}
			// This is necessary because it does not get compiled automatically by the GET
			exporterScraperConfig.TypeMeta.Kind = "ExporterScraperConfig"
			exporterScraperConfig.TypeMeta.APIVersion = "finops.krateo.io/v1"

			resource := &corev1.Service{}
			err = r.Get(ctx, testName+"-service", testNamespace, resource)
			if err != nil {
				t.Fatal(err)
			}

			resource.Spec.Type = corev1.ServiceTypeClusterIP

			err = r.Update(ctx, resource)
			if err != nil {
				t.Fatal(err)
			}

			time.Sleep(5 * time.Second) // Wait for informer to restore

			err = r.Get(ctx, testName+"-service", testNamespace, resource)
			if err != nil {
				t.Fatal(err)
			}

			resource.TypeMeta.Kind = "ExporterScraperConfig"
			resource.TypeMeta.APIVersion = "finops.krateo.io/v1"

			if !comparators.CheckService(*resource, *exporterScraperConfig) {
				t.Fatal("Service not restored by informer")
			}
			return ctx
		}).Feature()

	testenv.Test(t, create, delete, modify)
}

// startTestManager starts the controller manager with the given config
func startTestManager(ctx context.Context) error {
	os.Setenv("REGISTRY", "ghcr.io/krateoplatformops")
	os.Setenv("REGISTRY_CREDENTIALS", "registry-credentials")
	os.Setenv("EXPORTER_VERSION", "0.4.0")
	os.Setenv("RESOURCE_EXPORTER_VERSION", "0.4.0")

	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", true,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", false,
		"If set the metrics endpoint is served securely")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	disableHTTP2 := func(c *tls.Config) {
		c.NextProtos = []string{"http/1.1"}
	}

	tlsOpts := []func(*tls.Config){}
	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: tlsOpts,
	})
	namespaceCacheConfigMap := make(map[string]cache.Config)
	namespaceCacheConfigMap[testNamespace] = cache.Config{}
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:   metricsAddr,
			SecureServing: secureMetrics,
			TLSOpts:       tlsOpts,
		},
		WebhookServer:           webhookServer,
		HealthProbeBindAddress:  probeAddr,
		LeaderElection:          enableLeaderElection,
		LeaderElectionNamespace: testNamespace,
		LeaderElectionID:        "7867b561.krateo.io",
		Cache:                   cache.Options{DefaultNamespaces: namespaceCacheConfigMap},
	})
	if err != nil {
		return fmt.Errorf("failed to create manager: %w", err)
	}

	gv_apps := schema.GroupVersion{
		Group:   "apps",
		Version: "v1",
	}
	gv_core := schema.GroupVersion{
		Group:   "",
		Version: "v1",
	}
	cfg := ctrl.GetConfigOrDie()

	dynClient, err := clientHelper.New(cfg)
	if err != nil {
		return err
	}
	informerFactory := informer_factory.InformerFactory{
		DynClient: dynClient,
		Logger:    logging.NewLogrLogger(operatorlogger.Log.WithName("operator-exporter-informer")),
	}

	informerFactory.StartInformer(testNamespace, gv_apps.WithResource("deployments"))
	informerFactory.StartInformer(testNamespace, gv_core.WithResource("configmaps"))
	informerFactory.StartInformer(testNamespace, gv_core.WithResource("services"))

	o := controller.Options{
		Logger:                  logging.NewLogrLogger(operatorlogger.Log.WithName("operator-exporter")),
		MaxConcurrentReconciles: 1,
		PollInterval:            100,
		GlobalRateLimiter:       ratelimiter.NewGlobal(100),
	}

	if err := operatorexporter.Setup(mgr, o); err != nil {
		return err
	}

	// Start the manager
	go func() {
		if err := mgr.Start(ctx); err != nil {
			log.Error().Err(err).Msg("Failed to start manager")
		}
	}()

	return nil
}
