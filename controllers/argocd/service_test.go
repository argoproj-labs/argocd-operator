package argocd

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
	testclient "k8s.io/client-go/kubernetes/fake"

	"k8s.io/apimachinery/pkg/types"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	promoter "github.com/argoproj-labs/gitops-promoter/api/v1alpha1"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

func TestEnsureAutoTLSAnnotation(t *testing.T) {
	a := makeTestArgoCD()
	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
	fakeClient := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	t.Run("Ensure annotation will be set for OpenShift", func(t *testing.T) {
		argoutil.SetRouteAPIFound(true)
		svc := newService(a)

		// Annotation is inserted, update is required
		needUpdate, err := ensureAutoTLSAnnotation(fakeClient, svc, "some-secret", true)
		assert.Nil(t, err)
		assert.Equal(t, needUpdate, true)
		atls, ok := svc.Annotations[common.AnnotationOpenShiftServiceCA]
		assert.Equal(t, ok, true)
		assert.Equal(t, atls, "some-secret")

		// Annotation already set, doesn't need update
		needUpdate, err = ensureAutoTLSAnnotation(fakeClient, svc, "some-secret", true)
		assert.Nil(t, err)
		assert.Equal(t, needUpdate, false)
	})
	t.Run("Ensure annotation will be unset for OpenShift", func(t *testing.T) {
		argoutil.SetRouteAPIFound(true)
		svc := newService(a)
		svc.Annotations = make(map[string]string)
		svc.Annotations[common.AnnotationOpenShiftServiceCA] = "some-secret"

		// Annotation getting removed, update required
		needUpdate, err := ensureAutoTLSAnnotation(fakeClient, svc, "some-secret", false)
		assert.Nil(t, err)
		assert.Equal(t, needUpdate, true)
		_, ok := svc.Annotations[common.AnnotationOpenShiftServiceCA]
		assert.Equal(t, ok, false)

		// Annotation does not exist, no update required
		needUpdate, err = ensureAutoTLSAnnotation(fakeClient, svc, "some-secret", false)
		assert.Nil(t, err)
		assert.Equal(t, needUpdate, false)
	})
	t.Run("Ensure annotation will not be set for non-OpenShift", func(t *testing.T) {
		argoutil.SetRouteAPIFound(false)
		svc := newService(a)
		needUpdate, err := ensureAutoTLSAnnotation(fakeClient, svc, "some-secret", true)
		assert.Nil(t, err)
		assert.Equal(t, needUpdate, false)
		_, ok := svc.Annotations[common.AnnotationOpenShiftServiceCA]
		assert.Equal(t, ok, false)
	})
	t.Run("Ensure annotation will not be set if the TLS secret is already present", func(t *testing.T) {
		argoutil.SetRouteAPIFound(true)
		svc := newService(a)
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-secret",
				Namespace: svc.Namespace,
			},
		}
		err := fakeClient.Create(context.Background(), secret)
		assert.NoError(t, err)
		needUpdate, err := ensureAutoTLSAnnotation(fakeClient, svc, secret.Name, true)
		assert.Nil(t, err)
		assert.Equal(t, needUpdate, false)
		_, ok := svc.Annotations[common.AnnotationOpenShiftServiceCA]
		assert.Equal(t, ok, false)

		// Annotation does not exist, no update required
		needUpdate, err = ensureAutoTLSAnnotation(fakeClient, svc, "some-secret", false)
		assert.Nil(t, err)
		assert.Equal(t, needUpdate, false)
	})
	t.Run("Restore annotation when TLS secret exists and was created by OpenShift Service CA", func(t *testing.T) {
		argoutil.SetRouteAPIFound(true)
		svc := newService(a)
		svc.Name = "argocd-server"
		sec := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ArgoCDServerTLSSecretName,
				Namespace: svc.Namespace,
				Annotations: map[string]string{
					common.AnnotationOpenShiftOriginatingServiceName: svc.Name,
				},
				OwnerReferences: []metav1.OwnerReference{
					{Name: svc.Name, Kind: "Service"},
				},
			},
		}
		err := fakeClient.Create(context.Background(), sec)
		assert.NoError(t, err)
		needUpdate, err := ensureAutoTLSAnnotation(fakeClient, svc, sec.Name, true)
		assert.NoError(t, err)
		assert.True(t, needUpdate)
		atls, ok := svc.Annotations[common.AnnotationOpenShiftServiceCA]
		assert.True(t, ok)
		assert.Equal(t, sec.Name, atls)
		needUpdate, err = ensureAutoTLSAnnotation(fakeClient, svc, sec.Name, true)
		assert.NoError(t, err)
		assert.False(t, needUpdate)
	})
}

func TestReconcileServerService(t *testing.T) {
	a := makeTestArgoCD()
	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())
	a = makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Server.Service.Type = "NodePort"
	})
	serverService := newServiceWithSuffix("server", "server", a)
	t.Run("Server Service Created when the Server Service is not found ", func(t *testing.T) {
		err := r.Get(context.TODO(), types.NamespacedName{
			Name:      "argocd-server",
			Namespace: testNamespace,
		}, serverService)
		assert.True(t, errors.IsNotFound(err))

		err = r.reconcileServerService(a)
		assert.NoError(t, err)

		err = r.Get(context.TODO(), types.NamespacedName{
			Name:      "argocd-server",
			Namespace: testNamespace,
		}, serverService)
		assert.NoError(t, err)
		assert.Equal(t, a.Spec.Server.Service.Type, serverService.Spec.Type)
	})

	t.Run("Server Service Type update ", func(t *testing.T) {
		// Reconcile with previous existing Server Service with a different Type
		a.Spec.Server.Service.Type = "ClusterIP"
		assert.NotEqual(t, a.Spec.Server.Service.Type, serverService.Spec.Type)

		err := r.reconcileServerService(a)
		assert.NoError(t, err)

		// Existing Server is found and has the argoCD new Server Service Type
		err = r.Get(context.TODO(), types.NamespacedName{
			Name:      "argocd-server",
			Namespace: testNamespace,
		}, serverService)
		assert.NoError(t, err)
		assert.Equal(t, a.Spec.Server.Service.Type, serverService.Spec.Type)
	})
}

// If `remote` field is used in CR, then the component resources should not be created
func TestReconcileArgoCD_reconcileRedisWithRemoteEn(t *testing.T) {
	cr := makeTestArgoCD()

	resObjs := []client.Object{cr}
	subresObjs := []client.Object{cr}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	redisRemote := "https://remote.redis.instance"

	cr.Spec.Redis.Remote = &redisRemote
	assert.NoError(t, r.reconcileRedisService(cr))

	s := &corev1.Service{}

	assert.ErrorContains(t, r.Get(context.TODO(), types.NamespacedName{Name: cr.Name + "-redis", Namespace: cr.Namespace}, s),
		"services \"argocd-redis\" not found")
}

func TestReconcileArgoCD_reconcileRepoServerWithRemoteEnabled(t *testing.T) {
	cr := makeTestArgoCD()

	resObjs := []client.Object{cr}
	subresObjs := []client.Object{cr}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	repoServerRemote := "https://remote.repo-server.instance"

	cr.Spec.Repo.Remote = &repoServerRemote
	assert.NoError(t, r.reconcileRepoService(cr))

	s := &corev1.Service{}

	assert.ErrorContains(t, r.Get(context.TODO(), types.NamespacedName{Name: cr.Name + "-repo-server", Namespace: cr.Namespace}, s),
		"services \"argocd-repo-server\" not found")
}

func TestServiceWithLongName(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	// Create ArgoCD with a very long name that will trigger truncation
	longName := "this-is-a-very-long-argocd-instance-name-that-will-exceed-the-kubernetes-name-limit-and-require-truncation"
	a := makeTestArgoCD()
	a.Name = longName

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	// Test repo server service
	err := r.reconcileRepoService(a)
	assert.NoError(t, err)

	// Get all services and find the repo server service
	serviceList := &corev1.ServiceList{}
	err = r.List(context.TODO(), serviceList, client.InNamespace(a.Namespace))
	assert.NoError(t, err)

	var repoService *corev1.Service
	for i := range serviceList.Items {
		if serviceList.Items[i].Labels[common.ArgoCDKeyComponent] == "repo-server" {
			repoService = &serviceList.Items[i]
			break
		}
	}
	assert.NotNil(t, repoService, "Repo server service should exist")

	// Verify that the service name is truncated and within limits
	assert.LessOrEqual(t, len(repoService.Name), 63)
	assert.Contains(t, repoService.Name, "repo-server")

	// Verify that the labels are set correctly
	assert.Equal(t, repoService.Name, repoService.Labels[common.ArgoCDKeyName])
	assert.Equal(t, "repo-server", repoService.Labels[common.ArgoCDKeyComponent])

	// Verify that the selector matches the labels
	assert.Equal(t, repoService.Name, repoService.Spec.Selector[common.ArgoCDKeyName])
}

func TestEnsureMetricsServiceAnnotations(t *testing.T) {
	t.Run("Apply annotations to service without existing annotations", func(t *testing.T) {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "test-namespace",
			},
		}

		metricsSpec := &argoproj.ArgoCDMetricsSpec{
			Annotations: map[string]string{
				"ad.datadoghq.com/service.check_names":  `["openmetrics"]`,
				"ad.datadoghq.com/service.init_configs": `[{}]`,
				"custom.io/annotation":                  "value",
			},
		}

		modified := argoutil.EnsureMetricsServiceAnnotations(svc, metricsSpec)
		assert.True(t, modified)
		assert.Equal(t, `["openmetrics"]`, svc.Annotations["ad.datadoghq.com/service.check_names"])
		assert.Equal(t, `[{}]`, svc.Annotations["ad.datadoghq.com/service.init_configs"])
		assert.Equal(t, "value", svc.Annotations["custom.io/annotation"])
	})

	t.Run("Apply annotations to service with existing non-k8s annotations removes old annotations", func(t *testing.T) {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "test-namespace",
				Annotations: map[string]string{
					"existing.io/annotation": "existing-value",
				},
			},
		}

		metricsSpec := &argoproj.ArgoCDMetricsSpec{
			Annotations: map[string]string{
				"custom.io/annotation": "value",
			},
		}

		modified := argoutil.EnsureMetricsServiceAnnotations(svc, metricsSpec)
		assert.True(t, modified)
		assert.Equal(t, "value", svc.Annotations["custom.io/annotation"])
		// Old annotation should be removed since it's not in spec and not a k8s annotation
		_, hasExisting := svc.Annotations["existing.io/annotation"]
		assert.False(t, hasExisting)
	})

	t.Run("No modification when annotations already match", func(t *testing.T) {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "test-namespace",
				Annotations: map[string]string{
					"custom.io/annotation": "value",
				},
			},
		}

		metricsSpec := &argoproj.ArgoCDMetricsSpec{
			Annotations: map[string]string{
				"custom.io/annotation": "value",
			},
		}

		modified := argoutil.EnsureMetricsServiceAnnotations(svc, metricsSpec)
		assert.False(t, modified)
	})

	t.Run("Remove all annotations when metricsSpec is nil", func(t *testing.T) {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "test-namespace",
				Annotations: map[string]string{
					"custom.io/annotation": "value",
				},
			},
		}

		modified := argoutil.EnsureMetricsServiceAnnotations(svc, nil)
		assert.True(t, modified)
		// All non-k8s annotations should be removed
		_, hasCustom := svc.Annotations["custom.io/annotation"]
		assert.False(t, hasCustom)
	})

	t.Run("Remove all annotations when metricsSpec has no annotations", func(t *testing.T) {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "test-namespace",
				Annotations: map[string]string{
					"custom.io/annotation": "value",
				},
			},
		}

		metricsSpec := &argoproj.ArgoCDMetricsSpec{
			Interval: "30s",
		}

		modified := argoutil.EnsureMetricsServiceAnnotations(svc, metricsSpec)
		assert.True(t, modified)
		// All non-k8s annotations should be removed
		_, hasCustom := svc.Annotations["custom.io/annotation"]
		assert.False(t, hasCustom)
	})

	t.Run("Update existing annotation value", func(t *testing.T) {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "test-namespace",
				Annotations: map[string]string{
					"custom.io/annotation": "old-value",
				},
			},
		}

		metricsSpec := &argoproj.ArgoCDMetricsSpec{
			Annotations: map[string]string{
				"custom.io/annotation": "new-value",
			},
		}

		modified := argoutil.EnsureMetricsServiceAnnotations(svc, metricsSpec)
		assert.True(t, modified)
		assert.Equal(t, "new-value", svc.Annotations["custom.io/annotation"])
	})

	t.Run("Preserve Kubernetes-managed annotations", func(t *testing.T) {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "test-namespace",
				Annotations: map[string]string{
					"service.beta.openshift.io/serving-cert-secret-name": "my-secret",
					"kubernetes.io/service-name":                         "my-service",
					"custom.io/old-annotation":                           "old-value",
				},
			},
		}

		metricsSpec := &argoproj.ArgoCDMetricsSpec{
			Annotations: map[string]string{
				"custom.io/annotation": "value",
			},
		}

		modified := argoutil.EnsureMetricsServiceAnnotations(svc, metricsSpec)
		assert.True(t, modified)
		assert.Equal(t, "value", svc.Annotations["custom.io/annotation"])
		// K8s annotations should be preserved
		assert.Equal(t, "my-secret", svc.Annotations["service.beta.openshift.io/serving-cert-secret-name"])
		assert.Equal(t, "my-service", svc.Annotations["kubernetes.io/service-name"])
		// Old custom annotation should be removed
		_, hasOld := svc.Annotations["custom.io/old-annotation"]
		assert.False(t, hasOld)
	})

	t.Run("Remove annotation when deleted from spec", func(t *testing.T) {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "test-namespace",
				Annotations: map[string]string{
					"custom.io/annotation": "value",
					"to-be-removed":        "old-value",
				},
			},
		}

		metricsSpec := &argoproj.ArgoCDMetricsSpec{
			Annotations: map[string]string{
				"custom.io/annotation": "value",
			},
		}

		modified := argoutil.EnsureMetricsServiceAnnotations(svc, metricsSpec)
		assert.True(t, modified)
		assert.Equal(t, "value", svc.Annotations["custom.io/annotation"])
		_, hasRemoved := svc.Annotations["to-be-removed"]
		assert.False(t, hasRemoved)
	})
}

func TestMetricsAnnotationsPreserveOpenShiftAnnotations(t *testing.T) {
	t.Run("OpenShift service annotations are preserved during metrics reconciliation", func(t *testing.T) {
		// Start with a service that has OpenShift-managed annotations
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "test-namespace",
				Annotations: map[string]string{
					// OpenShift Service CA annotations that must be preserved
					common.AnnotationOpenShiftServiceCA:              "my-tls-secret",
					common.AnnotationOpenShiftOriginatingServiceName: "argocd-server",
					// Other Kubernetes annotations
					"kubernetes.io/service-name": "my-service",
					// User annotation that should be removed (not in spec)
					"user.io/old-annotation": "should-be-removed",
				},
			},
		}

		metricsSpec := &argoproj.ArgoCDMetricsSpec{
			Annotations: map[string]string{
				"ad.datadoghq.com/service.check_names": `["openmetrics"]`,
				"custom.io/monitoring":                 "enabled",
			},
		}

		// Apply metrics annotations
		modified := argoutil.EnsureMetricsServiceAnnotations(svc, metricsSpec)
		assert.True(t, modified)

		// Verify custom metrics annotations were added
		assert.Equal(t, `["openmetrics"]`, svc.Annotations["ad.datadoghq.com/service.check_names"])
		assert.Equal(t, "enabled", svc.Annotations["custom.io/monitoring"])

		// Verify OpenShift annotations were preserved
		assert.Equal(t, "my-tls-secret", svc.Annotations[common.AnnotationOpenShiftServiceCA],
			"OpenShift serving-cert-secret-name annotation must be preserved")
		assert.Equal(t, "argocd-server", svc.Annotations[common.AnnotationOpenShiftOriginatingServiceName],
			"OpenShift originating-service-name annotation must be preserved")

		// Verify other Kubernetes annotations were preserved
		assert.Equal(t, "my-service", svc.Annotations["kubernetes.io/service-name"])

		// Verify user annotation not in spec was removed
		_, hasOld := svc.Annotations["user.io/old-annotation"]
		assert.False(t, hasOld, "User annotations not in spec should be removed")
	})
}

func TestReconcileMetricsService_UpdatesAnnotationsOnExistingService(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Controller.Metrics = &argoproj.ArgoCDMetricsSpec{
			Annotations: map[string]string{
				"monitoring.io/scrape": "true",
				"monitoring.io/port":   "8082",
			},
		}
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	// Create the metrics service without annotations
	svc := newServiceWithSuffix("metrics", "metrics", a)
	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: applicationControllerResourceName(a),
	}
	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:     "metrics",
			Port:     8082,
			Protocol: corev1.ProtocolTCP,
		},
	}
	assert.NoError(t, cl.Create(context.TODO(), svc))

	// Reconcile should update the service with annotations
	assert.NoError(t, r.reconcileMetricsService(a))

	// Fetch the updated service
	updatedSvc := &corev1.Service{}
	assert.NoError(t, cl.Get(context.TODO(), types.NamespacedName{
		Name:      svc.Name,
		Namespace: svc.Namespace,
	}, updatedSvc))

	// Verify annotations were added
	assert.Equal(t, "true", updatedSvc.Annotations["monitoring.io/scrape"])
	assert.Equal(t, "8082", updatedSvc.Annotations["monitoring.io/port"])
}

func TestReconcileServerMetricsService_UpdatesAnnotationsOnExistingService(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Server.Metrics = &argoproj.ArgoCDMetricsSpec{
			Annotations: map[string]string{
				"datadog.io/scrape": "true",
				"custom.io/team":    "platform",
			},
		}
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	// Create the server metrics service without annotations
	svc := newServiceWithSuffix("server-metrics", "server", a)
	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: nameWithSuffix("server", a),
	}
	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:     "metrics",
			Port:     8083,
			Protocol: corev1.ProtocolTCP,
		},
	}
	assert.NoError(t, cl.Create(context.TODO(), svc))

	// Reconcile should update the service with annotations
	assert.NoError(t, r.reconcileServerMetricsService(a))

	// Fetch the updated service
	updatedSvc := &corev1.Service{}
	assert.NoError(t, cl.Get(context.TODO(), types.NamespacedName{
		Name:      svc.Name,
		Namespace: svc.Namespace,
	}, updatedSvc))

	// Verify annotations were added
	assert.Equal(t, "true", updatedSvc.Annotations["datadog.io/scrape"])
	assert.Equal(t, "platform", updatedSvc.Annotations["custom.io/team"])
}

func TestReconcileRepoService_UpdatesBothTLSAndMetricsAnnotations(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Repo.AutoTLS = "openshift"
		a.Spec.Repo.Metrics = &argoproj.ArgoCDMetricsSpec{
			Annotations: map[string]string{
				"prometheus.io/scrape": "true",
				"prometheus.io/path":   "/metrics",
			},
		}
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	// Enable Route API to allow AutoTLS
	argoutil.SetRouteAPIFound(true)
	defer argoutil.SetRouteAPIFound(false)

	// Create the repo server service without annotations
	svc := newServiceWithSuffix("repo-server", "repo-server", a)
	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: nameWithSuffix("repo-server", a),
	}
	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:     "server",
			Port:     8081,
			Protocol: corev1.ProtocolTCP,
		},
		{
			Name:     "metrics",
			Port:     8084,
			Protocol: corev1.ProtocolTCP,
		},
	}
	assert.NoError(t, cl.Create(context.TODO(), svc))

	// Reconcile should update the service with both TLS and metrics annotations
	assert.NoError(t, r.reconcileRepoService(a))

	// Fetch the updated service
	updatedSvc := &corev1.Service{}
	assert.NoError(t, cl.Get(context.TODO(), types.NamespacedName{
		Name:      svc.Name,
		Namespace: svc.Namespace,
	}, updatedSvc))

	// Verify TLS annotation was added
	assert.Equal(t, common.ArgoCDRepoServerTLSSecretName, updatedSvc.Annotations[common.AnnotationOpenShiftServiceCA])

	// Verify metrics annotations were added
	assert.Equal(t, "true", updatedSvc.Annotations["prometheus.io/scrape"])
	assert.Equal(t, "/metrics", updatedSvc.Annotations["prometheus.io/path"])
}
