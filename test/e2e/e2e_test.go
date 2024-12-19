package test

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	"github.com/krateoplatformops/finops-operator-exporter/internal/helpers/kube/comparators"
	"github.com/krateoplatformops/finops-operator-exporter/internal/utils"
)

type contextKey string

var (
	testenv env.Environment
)

const (
	testNamespace   = "finops-test" // If you changed this test environment, you need to change the RoleBinding in the "deploymentsPath" folder
	crdsPath1       = "../../config/crd/bases"
	crdsPath2       = "./manifests/crds"
	deploymentsPath = "./manifests/deployments"
	toTest          = "./manifests/to_test/"

	testName = "exporterscraperconfig-sample"
)

func TestMain(m *testing.M) {
	testenv = env.New()
	kindClusterName := "krateo-test"

	// Use pre-defined environment funcs to create a kind cluster prior to test run
	testenv.Setup(
		envfuncs.CreateCluster(kind.NewProvider(), kindClusterName),
		envfuncs.CreateNamespace(testNamespace),
		envfuncs.SetupCRDs(crdsPath1, "*"),
		envfuncs.SetupCRDs(crdsPath2, "*"),
	)

	// Use pre-defined environment funcs to teardown kind cluster after tests
	testenv.Finish(
		envfuncs.DeleteNamespace(testNamespace),
		envfuncs.TeardownCRDs(crdsPath1, "*"),
		envfuncs.TeardownCRDs(crdsPath2, "*"),
		envfuncs.DestroyCluster(kindClusterName),
	)

	// launch package tests
	os.Exit(testenv.Run(m))
}

func TestExporter(t *testing.T) {
	controllerCreationSig := make(chan bool, 2)
	create := features.New("Create").
		WithLabel("type", "CR and resources").
		Setup(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			r, err := resources.New(c.Client().RESTConfig())
			if err != nil {
				t.Fail()
			}
			operatorexporterapi.AddToScheme(r.GetScheme())
			r.WithNamespace(testNamespace)

			ctx = context.WithValue(ctx, contextKey("client"), r)

			err = decoder.DecodeEachFile(
				ctx, os.DirFS(deploymentsPath), "*",
				decoder.CreateHandler(r),
				decoder.MutateNamespace(testNamespace),
			)
			if err != nil {
				t.Fatalf("Failed due to error: %s", err)
			}

			err = decoder.DecodeEachFile(
				ctx, os.DirFS(toTest), "*",
				decoder.CreateHandler(r),
				decoder.MutateNamespace(testNamespace),
			)
			if err != nil {
				t.Fatalf("Failed due to error: %s", err)
			}

			if err := wait.For(
				conditions.New(r).DeploymentAvailable("finops-operator-exporter-controller-manager", testNamespace),
				wait.WithTimeout(45*time.Second),
				wait.WithInterval(5*time.Second),
			); err != nil {
				log.Printf("Timed out while waiting for finops-operator-exporter deployment: %s", err)
			}
			controllerCreationSig <- true
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

			select {
			case <-time.After(55 * time.Second):
				t.Fatal("Timed out wating for controller creation")
			case created := <-controllerCreationSig:
				if !created {
					t.Fatal("Operator deployment not ready")
				}
			}

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

			return ctx
		}).Feature()

	delete := features.New("Delete").
		WithLabel("type", "Resources").
		Setup(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			r, err := resources.New(c.Client().RESTConfig())
			if err != nil {
				t.Fail()
			}
			operatorexporterapi.AddToScheme(r.GetScheme())
			r.WithNamespace(testNamespace)

			ctx = context.WithValue(ctx, contextKey("client"), r)

			return ctx
		}).
		Assess("Deployment", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			r := ctx.Value(contextKey("client")).(*resources.Resources)

			resource := &appsv1.Deployment{
				ObjectMeta: v1.ObjectMeta{
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
				ObjectMeta: v1.ObjectMeta{
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
				ObjectMeta: v1.ObjectMeta{
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
			r, err := resources.New(c.Client().RESTConfig())
			if err != nil {
				t.Fail()
			}
			operatorexporterapi.AddToScheme(r.GetScheme())
			r.WithNamespace(testNamespace)

			ctx = context.WithValue(ctx, contextKey("client"), r)

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

	// test feature
	testenv.Test(t, create, delete, modify)
}
