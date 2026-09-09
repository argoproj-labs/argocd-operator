package argocd

import (
	"context"
	"reflect"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/stretchr/testify/assert"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"

	corev1 "k8s.io/api/core/v1"
	testclient "k8s.io/client-go/kubernetes/fake"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	promoter "github.com/argoproj-labs/gitops-promoter/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestReconcileArgoCD_clusterRoleBindingMapper(t *testing.T) {
	type fields struct {
		client client.Client
		scheme *runtime.Scheme
	}
	type args struct {
		o client.Object
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []reconcile.Request
	}{
		{
			name:   "crb incorrectly annotated",
			fields: fields{},
			args: args{
				&rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"argocds.argoproj.io/name": "foo",
							"foo/namespace":            "foo-ns",
						},
					},
				},
			},
			want: []reconcile.Request{},
		},
		{
			name:   "crb associated with ArgoCD",
			fields: fields{},
			args: args{
				&rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"argocds.argoproj.io/name":      "foo",
							"argocds.argoproj.io/namespace": "foo-ns",
						},
					},
				},
			},
			want: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      "foo",
						Namespace: "foo-ns",
					},
				},
			},
		},
		{
			name:   "crb not associated with ArgoCD",
			fields: fields{},
			args: args{
				&rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"foo/name":      "foo",
							"foo/namespace": "foo-ns",
						},
					},
				},
			},

			want: []reconcile.Request{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ReconcileArgoCD{
				Client: tt.fields.client,
				Scheme: tt.fields.scheme,
			}
			if got := r.clusterResourceMapper(context.TODO(), tt.args.o); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReconcileArgoCD.clusterRoleBindingMapper() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReconcileArgoCD_tlsSecretMapperRepoServer(t *testing.T) {
	argocd := &argoproj.ArgoCD{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd",
			Namespace: "argocd-operator",
			UID:       "abcd",
		},
	}

	t.Run("Map with proper ownerReference", func(t *testing.T) {
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-repo-server",
				Namespace: "argocd-operator",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "argoproj.io/v1alpha1",
						Kind:       "ArgoCD",
						Name:       "argocd",
						UID:        argocd.GetUID(),
					},
				},
				UID: "service-123",
			},
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-repo-server-tls",
				Namespace: "argocd-operator",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Kind:       "Service",
						Name:       "argocd-repo-server",
						UID:        service.GetUID(),
					},
				},
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{
				corev1.TLSCertKey:       []byte("foo"),
				corev1.TLSPrivateKeyKey: []byte("bar"),
			},
		}

		resObjs := []client.Object{argocd, secret, service}
		subresObjs := []client.Object{argocd}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme, configv1.Install, routev1.Install)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

		want := []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      "argocd",
					Namespace: "argocd-operator",
				},
			},
		}
		got := r.tlsSecretMapper(context.TODO(), secret)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Reconciliation unsucessful: got: %v, want: %v", got, want)
		}
	})

	t.Run("Map with ownerReference on non-existing owner", func(t *testing.T) {
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-repo-server",
				Namespace: "argocd-operator",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "argoproj.io/v1alpha1",
						Kind:       "ArgoCD",
						Name:       "argocd",
						UID:        argocd.GetUID(),
					},
				},
				UID: "service-123",
			},
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-repo-server-tls",
				Namespace: "argocd-operator",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Kind:       "Service",
						Name:       "argocd-repo-server",
						UID:        service.GetUID(),
					},
				},
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{
				corev1.TLSCertKey:       []byte("foo"),
				corev1.TLSPrivateKeyKey: []byte("bar"),
			},
		}

		resObjs := []client.Object{argocd, secret}
		subresObjs := []client.Object{argocd}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme, configv1.Install, routev1.Install)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

		want := []reconcile.Request{}
		got := r.tlsSecretMapper(context.TODO(), secret)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Reconciliation unsucessful: got: %v, want: %v", got, want)
		}
	})

	t.Run("Map with invalid owner", func(t *testing.T) {
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-repo-server",
				Namespace: "argocd-operator",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "argoproj.io/v1alpha1",
						Kind:       "ArgoCD",
						Name:       "argocd",
						UID:        argocd.GetUID(),
					},
				},
				UID: "service-123",
			},
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-repo-server-tls",
				Namespace: "argocd-operator",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Kind:       "Service",
						Name:       "argocd-server",
						UID:        service.GetUID(),
					},
				},
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{
				corev1.TLSCertKey:       []byte("foo"),
				corev1.TLSPrivateKeyKey: []byte("bar"),
			},
		}

		resObjs := []client.Object{argocd, secret, service}
		subresObjs := []client.Object{argocd}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme, configv1.Install, routev1.Install)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

		want := []reconcile.Request{}
		got := r.tlsSecretMapper(context.TODO(), secret)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Reconciliation unsucessful: got: %v, want: %v", got, want)
		}
	})

	t.Run("Map with owner annotation", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-repo-server-tls",
				Namespace: "argocd-operator",
				Annotations: map[string]string{
					common.AnnotationName: "argocd",
				},
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{
				corev1.TLSCertKey:       []byte("foo"),
				corev1.TLSPrivateKeyKey: []byte("bar"),
			},
		}

		resObjs := []client.Object{argocd, secret}
		subresObjs := []client.Object{argocd}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme, configv1.Install, routev1.Install)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

		want := []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      "argocd",
					Namespace: "argocd-operator",
				},
			},
		}
		got := r.tlsSecretMapper(context.TODO(), secret)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Reconciliation unsucessful: got: %v, want: %v", got, want)
		}
	})

	t.Run("Map without owner and without annotation", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-repo-server-tls",
				Namespace: "argocd-operator",
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{
				corev1.TLSCertKey:       []byte("foo"),
				corev1.TLSPrivateKeyKey: []byte("bar"),
			},
		}

		resObjs := []client.Object{argocd, secret}
		subresObjs := []client.Object{argocd}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme, configv1.Install, routev1.Install)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

		want := []reconcile.Request{}
		got := r.tlsSecretMapper(context.TODO(), secret)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Reconciliation unsucessful: got: %v, want: %v", got, want)
		}
	})
}

func TestReconcileArgoCD_tlsSecretMapperRedis(t *testing.T) {
	argocd := &argoproj.ArgoCD{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd",
			Namespace: "argocd-operator",
			UID:       "abcd",
		},
	}

	t.Run("Map with proper ownerReference", func(t *testing.T) {
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-redis",
				Namespace: "argocd-operator",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "argoproj.io/v1alpha1",
						Kind:       "ArgoCD",
						Name:       "argocd",
						UID:        argocd.GetUID(),
					},
				},
				UID: "service-123",
			},
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-operator-redis-tls",
				Namespace: "argocd-operator",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Kind:       "Service",
						Name:       "argocd-redis",
						UID:        service.GetUID(),
					},
				},
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{
				corev1.TLSCertKey:       []byte("foo"),
				corev1.TLSPrivateKeyKey: []byte("bar"),
			},
		}

		resObjs := []client.Object{argocd, secret, service}
		subresObjs := []client.Object{argocd}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme, configv1.Install, routev1.Install)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

		want := []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      "argocd",
					Namespace: "argocd-operator",
				},
			},
		}
		got := r.tlsSecretMapper(context.TODO(), secret)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Reconciliation unsucessful: got: %v, want: %v", got, want)
		}
	})

	t.Run("Map with ownerReference on non-existing owner", func(t *testing.T) {
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-redis",
				Namespace: "argocd-operator",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "argoproj.io/v1alpha1",
						Kind:       "ArgoCD",
						Name:       "argocd",
						UID:        argocd.GetUID(),
					},
				},
				UID: "service-123",
			},
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-operator-redis-tls",
				Namespace: "argocd-operator",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Kind:       "Service",
						Name:       "argocd-redis",
						UID:        service.GetUID(),
					},
				},
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{
				corev1.TLSCertKey:       []byte("foo"),
				corev1.TLSPrivateKeyKey: []byte("bar"),
			},
		}

		resObjs := []client.Object{argocd, secret}
		subresObjs := []client.Object{argocd}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme, configv1.Install, routev1.Install)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

		want := []reconcile.Request{}
		got := r.tlsSecretMapper(context.TODO(), secret)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Reconciliation unsucessful: got: %v, want: %v", got, want)
		}
	})

	t.Run("Map with invalid owner", func(t *testing.T) {
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-redis",
				Namespace: "argocd-operator",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "argoproj.io/v1alpha1",
						Kind:       "ArgoCD",
						Name:       "argocd",
						UID:        argocd.GetUID(),
					},
				},
				UID: "service-123",
			},
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-operator-redis-tls",
				Namespace: "argocd-operator",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Kind:       "Service",
						Name:       "argocd-server",
						UID:        service.GetUID(),
					},
				},
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{
				corev1.TLSCertKey:       []byte("foo"),
				corev1.TLSPrivateKeyKey: []byte("bar"),
			},
		}

		resObjs := []client.Object{argocd, secret, service}
		subresObjs := []client.Object{argocd}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme, configv1.Install, routev1.Install)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

		want := []reconcile.Request{}
		got := r.tlsSecretMapper(context.TODO(), secret)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Reconciliation unsucessful: got: %v, want: %v", got, want)
		}
	})

	t.Run("Map with owner annotation", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-operator-redis-tls",
				Namespace: "argocd-operator",
				Annotations: map[string]string{
					common.AnnotationName: "argocd",
				},
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{
				corev1.TLSCertKey:       []byte("foo"),
				corev1.TLSPrivateKeyKey: []byte("bar"),
			},
		}

		resObjs := []client.Object{argocd, secret}
		subresObjs := []client.Object{argocd}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme, configv1.Install, routev1.Install)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

		want := []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      "argocd",
					Namespace: "argocd-operator",
				},
			},
		}
		got := r.tlsSecretMapper(context.TODO(), secret)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Reconciliation unsucessful: got: %v, want: %v", got, want)
		}
	})

	t.Run("Map without owner and without annotation", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-operator-redis-tls",
				Namespace: "argocd-operator",
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{
				corev1.TLSCertKey:       []byte("foo"),
				corev1.TLSPrivateKeyKey: []byte("bar"),
			},
		}

		resObjs := []client.Object{argocd, secret}
		subresObjs := []client.Object{argocd}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme, configv1.Install, routev1.Install)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

		want := []reconcile.Request{}
		got := r.tlsSecretMapper(context.TODO(), secret)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Reconciliation unsucessful: got: %v, want: %v", got, want)
		}
	})
}

func TestReconcileArgoCD_namespaceResourceMapperWithManagedByLabel(t *testing.T) {
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	a.Namespace = "newTestNamespace"

	// Fake client returns an error if ResourceVersion is not nil
	a.ResourceVersion = ""
	assert.NoError(t, r.Create(context.TODO(), a))

	type test struct {
		name string
		o    client.Object
		want []reconcile.Request
	}

	tests := []test{
		{
			name: "test when namespace is labelled",
			o: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testNamespace",
					Labels: map[string]string{
						common.ArgoCDManagedByLabel: a.Namespace,
					},
				},
			},
			want: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      a.Name,
						Namespace: a.Namespace,
					},
				},
			},
		},
		{
			name: "test when namespace is not labelled",
			o: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "testNamespace",
					Labels: make(map[string]string),
				},
			},
			want: []reconcile.Request{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := r.namespaceResourceMapper(context.TODO(), tt.o); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReconcileArgoCD.namespaceResourceMapper(), got = %v, want = %v", got, tt.want)
			}
		})
	}
}

func TestReconcileArgoCD_namespaceResourceMapperForSpecificNamespaceWithoutManagedByLabel(t *testing.T) {
	argocd1 := makeTestArgoCD()
	resObjs := []client.Object{argocd1}
	subresObjs := []client.Object{argocd1}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	argocd1.Name = "argocd1"
	argocd1.Namespace = "argo-test-1"
	argocd1.Spec.SourceNamespaces = append(argocd1.Spec.SourceNamespaces, "test-namespace-1")
	// Fake client returns an error if ResourceVersion is not nil
	argocd1.ResourceVersion = ""

	assert.NoError(t, r.Create(context.TODO(), argocd1))

	type test struct {
		name string
		o    client.Object
		want []reconcile.Request
	}

	tests := []test{
		{
			name: "Reconcile for Namespace 'test-namespace-1'",
			o: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-namespace-1",
				},
			},
			want: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      argocd1.Name,
						Namespace: argocd1.Namespace,
					},
				},
			},
		},
		{
			name: "No Reconcile for Namespace 'test-namespace-2'",
			o: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-namespace-2",
				},
			},
			want: []reconcile.Request{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := r.namespaceResourceMapper(context.TODO(), tt.o); !assert.ElementsMatch(t, got, tt.want) {
				t.Errorf("ReconcileArgoCD.sourceNamespaceMapper(), got = %v, want = %v", got, tt.want)
			}
		})
	}
}

func TestReconcileArgoCD_namespaceResourceMapperForWildCardPatternNamespaceWithoutManagedByLabel(t *testing.T) {
	argocd1 := makeTestArgoCD()
	resObjs := []client.Object{argocd1}
	subresObjs := []client.Object{argocd1}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	argocd1.Name = "argocd1"
	argocd1.Namespace = "argo-test-1"
	argocd1.Spec.SourceNamespaces = append(argocd1.Spec.SourceNamespaces, "test*")
	// Fake client returns an error if ResourceVersion is not nil
	argocd1.ResourceVersion = ""

	assert.NoError(t, r.Create(context.TODO(), argocd1))

	type test struct {
		name string
		o    client.Object
		want []reconcile.Request
	}

	tests := []test{
		{
			name: "Reconcile for Namespace 'test-namespace-1'",
			o: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-namespace-1",
				},
			},
			want: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      argocd1.Name,
						Namespace: argocd1.Namespace,
					},
				},
			},
		},
		{
			name: "Reconcile for Namespace 'test-namespace-2'",
			o: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-namespace-2",
				},
			},
			want: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      argocd1.Name,
						Namespace: argocd1.Namespace,
					},
				},
			},
		},
		{
			name: "No Reconcile for Namespace 'prod-namespace-1'",
			o: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "prod-namespace-1",
				},
			},
			want: []reconcile.Request{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := r.namespaceResourceMapper(context.TODO(), tt.o); !assert.ElementsMatch(t, got, tt.want) {
				t.Errorf("ReconcileArgoCD.sourceNamespaceMapper(), got = %v, want = %v", got, tt.want)
			}
		})
	}
}

func TestReconcileArgoCD_namespaceResourceMapperForMultipleSourceNamespacesWithoutManagedByLabel(t *testing.T) {
	argocd1 := makeTestArgoCD()
	resObjs := []client.Object{argocd1}
	subresObjs := []client.Object{argocd1}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	argocd1.Name = "argocd1"
	argocd1.Namespace = "argo-test-1"
	argocd1.Spec.SourceNamespaces = append(argocd1.Spec.SourceNamespaces, "test*", "dev*")
	// Fake client returns an error if ResourceVersion is not nil
	argocd1.ResourceVersion = ""

	assert.NoError(t, r.Create(context.TODO(), argocd1))

	type test struct {
		name string
		o    client.Object
		want []reconcile.Request
	}

	tests := []test{
		{
			name: "Reconcile for Namespace 'test-namespace-1'",
			o: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-namespace-1",
				},
			},
			want: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      argocd1.Name,
						Namespace: argocd1.Namespace,
					},
				},
			},
		},
		{
			name: "Reconcile for Namespace 'test-namespace-2'",
			o: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-namespace-2",
				},
			},
			want: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      argocd1.Name,
						Namespace: argocd1.Namespace,
					},
				},
			},
		},
		{
			name: "Reconcile for Namespace 'dev-namespace-1'",
			o: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "dev-namespace-1",
				},
			},
			want: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      argocd1.Name,
						Namespace: argocd1.Namespace,
					},
				},
			},
		},
		{
			name: "No Reconcile for Namespace 'prod-namespace-1'",
			o: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "prod-namespace-1",
				},
			},
			want: []reconcile.Request{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := r.namespaceResourceMapper(context.TODO(), tt.o); !assert.ElementsMatch(t, got, tt.want) {
				t.Errorf("ReconcileArgoCD.sourceNamespaceMapper(), got = %v, want = %v", got, tt.want)
			}
		})
	}
}

func TestReconcileArgoCD_namespaceResourceMapperForWildCardNamespaceWithoutManagedByLabel(t *testing.T) {
	argocd1 := makeTestArgoCD()
	resObjs := []client.Object{argocd1}
	subresObjs := []client.Object{argocd1}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	argocd1.Name = "argocd1"
	argocd1.Namespace = "argo-test-1"
	argocd1.Spec.SourceNamespaces = append(argocd1.Spec.SourceNamespaces, "*")
	// Fake client returns an error if ResourceVersion is not nil
	argocd1.ResourceVersion = ""

	assert.NoError(t, r.Create(context.TODO(), argocd1))

	type test struct {
		name string
		o    client.Object
		want []reconcile.Request
	}

	tests := []test{
		{
			name: "Reconcile for Namespace 'test-namespace-1'",
			o: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-namespace-1",
				},
			},
			want: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      argocd1.Name,
						Namespace: argocd1.Namespace,
					},
				},
			},
		},
		{
			name: "Reconcile for Namespace 'test-namespace-2'",
			o: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-namespace-2",
				},
			},
			want: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      argocd1.Name,
						Namespace: argocd1.Namespace,
					},
				},
			},
		},
		{
			name: "Reconcile for Namespace 'prod-namespace-1'",
			o: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "prod-namespace-1",
				},
			},
			want: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      argocd1.Name,
						Namespace: argocd1.Namespace,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := r.namespaceResourceMapper(context.TODO(), tt.o); !assert.ElementsMatch(t, got, tt.want) {
				t.Errorf("ReconcileArgoCD.sourceNamespaceMapper(), got = %v, want = %v", got, tt.want)
			}
		})
	}
}

func TestReconcileArgoCD_tlsSecretMapperUserManagedSecret(t *testing.T) {
	emptyReq := []reconcile.Request{}
	reconcileReq := []reconcile.Request{{
		NamespacedName: client.ObjectKey{
			Name:      testArgoCDName,
			Namespace: testNamespace,
		},
	}}

	tests := []struct {
		name        string
		argocd      *argoproj.ArgoCD
		expectedReq []reconcile.Request
	}{
		{
			name: "tls secret for Server in ArgoCD CR",
			argocd: makeArgoCD(func(a *argoproj.ArgoCD) {
				a.Spec.Server.Route.TLS = &routev1.TLSConfig{
					ExternalCertificate: &routev1.LocalObjectReference{
						Name: "user-tls",
					},
				}
			}),
			expectedReq: reconcileReq,
		},
		{
			name: "tls secret for Prometheus in ArgoCD CR",
			argocd: makeArgoCD(func(a *argoproj.ArgoCD) {
				a.Spec.Prometheus.Route.TLS = &routev1.TLSConfig{
					ExternalCertificate: &routev1.LocalObjectReference{
						Name: "user-tls",
					},
				}
			}),
			expectedReq: reconcileReq,
		},
		{
			name: "tls secret for ApplicationSet in ArgoCD CR",
			argocd: makeArgoCD(func(a *argoproj.ArgoCD) {
				a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{}
				a.Spec.ApplicationSet.WebhookServer.Route.TLS = &routev1.TLSConfig{
					ExternalCertificate: &routev1.LocalObjectReference{
						Name: "user-tls",
					},
				}
			}),
			expectedReq: reconcileReq,
		},
		{
			name: "tls secret not referenced in ArgoCD CR",
			argocd: makeArgoCD(func(a *argoproj.ArgoCD) {
				a.Spec.Server.Route.Enabled = true
			}),
			expectedReq: emptyReq,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resObjs := []client.Object{test.argocd}
			subresObjs := []client.Object{test.argocd}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme, configv1.Install, routev1.Install)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "user-tls",
					Namespace: testNamespace,
				},
				Type: corev1.SecretTypeTLS,
				Data: map[string][]byte{
					corev1.TLSCertKey:       []byte("Y2VydGlmY2F0ZQ=="),
					corev1.TLSPrivateKeyKey: []byte("cHJpdmF0ZS1rZXk="),
				},
			}

			req := r.tlsSecretMapper(context.TODO(), secret)
			assert.Equal(t, test.expectedReq, req)
		})
	}
}

func TestReconcileArgoCD_nmMapper(t *testing.T) {
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	// Fake client returns an error if ResourceVersion is not nil
	a.ResourceVersion = ""

	expected := []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      a.Name,
				Namespace: a.Namespace,
			},
		},
	}

	// testing to check if it reconcile every argocd instance despite ManagedBy field is set or not
	tests := []struct {
		name string
		o    client.Object
		want []reconcile.Request
	}{
		{
			name: "ManagedBy set",
			o: &argoproj.NamespaceManagement{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-nsmgmt-1",
					Namespace: "ns1",
				},
				Spec: argoproj.NamespaceManagementSpec{
					ManagedBy: "some-ns",
				},
			},
			want: expected,
		},
		{
			name: "ManagedBy empty",
			o: &argoproj.NamespaceManagement{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-nsmgmt-2",
					Namespace: "ns2",
				},
				Spec: argoproj.NamespaceManagementSpec{
					ManagedBy: "",
				},
			},
			want: expected,
		},
		{
			name: "ManagedBy non-matching",
			o: &argoproj.NamespaceManagement{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-nsmgmt-3",
					Namespace: "ns3",
				},
				Spec: argoproj.NamespaceManagementSpec{
					ManagedBy: "another-ns",
				},
			},
			want: expected,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.nmMapper(context.TODO(), tt.o)
			assert.ElementsMatch(t, tt.want, got, "expected ArgoCD reconcile requests")
		})
	}
}

func TestReconcileArgoCD_imageUpdaterWatchNSMapper(t *testing.T) {
	argocd1 := makeTestArgoCD()
	argocd1.Name = "argocd1"
	argocd1.Namespace = "argo-ns-1"
	argocd1.Spec.ImageUpdater.Enabled = true
	argocd1.Spec.ImageUpdater.Env = []corev1.EnvVar{
		{Name: "IMAGE_UPDATER_WATCH_NAMESPACES", Value: "app-*"},
	}

	argocd2 := makeTestArgoCD()
	argocd2.Name = "argocd2"
	argocd2.Namespace = "argo-ns-2"
	argocd2.Spec.ImageUpdater.Enabled = true
	argocd2.Spec.ImageUpdater.Env = []corev1.EnvVar{
		{Name: "IMAGE_UPDATER_WATCH_NAMESPACES", Value: "/^team-[a-z]+$/"},
	}

	argocd3 := makeTestArgoCD()
	argocd3.Name = "argocd3"
	argocd3.Namespace = "argo-ns-3"
	argocd3.Spec.ImageUpdater.Enabled = false
	argocd3.Spec.ImageUpdater.Env = []corev1.EnvVar{
		{Name: "IMAGE_UPDATER_WATCH_NAMESPACES", Value: "app-*"},
	}

	argocd4 := makeTestArgoCD()
	argocd4.Name = "argocd4"
	argocd4.Namespace = "argo-ns-4"
	argocd4.Spec.ImageUpdater.Enabled = true
	// No IMAGE_UPDATER_WATCH_NAMESPACES → skip

	argocd5 := makeTestArgoCD()
	argocd5.Name = "argocd5"
	argocd5.Namespace = "argo-ns-5"
	argocd5.Spec.ImageUpdater.Enabled = true
	argocd5.Spec.ImageUpdater.Env = []corev1.EnvVar{
		{Name: "IMAGE_UPDATER_WATCH_NAMESPACES", Value: "*"},
	}

	for _, a := range []*argoproj.ArgoCD{argocd1, argocd2, argocd3, argocd4, argocd5} {
		a.ResourceVersion = ""
	}

	resObjs := []client.Object{argocd1, argocd2, argocd3, argocd4, argocd5}
	subresObjs := []client.Object{argocd1, argocd2, argocd3, argocd4, argocd5}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	req1 := reconcile.Request{NamespacedName: types.NamespacedName{Name: "argocd1", Namespace: "argo-ns-1"}}
	req2 := reconcile.Request{NamespacedName: types.NamespacedName{Name: "argocd2", Namespace: "argo-ns-2"}}

	tests := []struct {
		name      string
		namespace string
		want      []reconcile.Request
	}{
		{
			name:      "glob match triggers reconcile for argocd1",
			namespace: "app-frontend",
			want:      []reconcile.Request{req1},
		},
		{
			name:      "glob non-match triggers no reconcile",
			namespace: "other-ns",
			want:      []reconcile.Request{},
		},
		{
			name:      "regex match triggers reconcile for argocd2",
			namespace: "team-alpha",
			want:      []reconcile.Request{req2},
		},
		{
			name:      "regex non-match (digits not lower-alpha) triggers no reconcile",
			namespace: "team-123",
			want:      []reconcile.Request{},
		},
		{
			name:      "disabled image updater is ignored",
			namespace: "app-ignored",
			// argocd1 has app-* and is enabled; argocd3 has app-* but is disabled
			want: []reconcile.Request{req1},
		},
		{
			name:      "argocd with no IMAGE_UPDATER_WATCH_NAMESPACES is ignored",
			namespace: "any-namespace",
			// argocd4 is enabled but has no env var
			want: []reconcile.Request{},
		},
		{
			name:      "wildcard '*' is skipped (cluster-wide mode, no per-namespace matching)",
			namespace: "app-star-match",
			// argocd1 matches app-*, argocd5 has '*' which is skipped
			want: []reconcile.Request{req1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: tt.namespace}}
			got := r.imageUpdaterWatchNSMapper(context.TODO(), ns)
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}

// TestReconcileArgoCD_imageUpdaterWatchNSMapper_EmptyPatterns verifies that a trailing
// or leading comma in IMAGE_UPDATER_WATCH_NAMESPACES does not produce a match-all empty
// pattern, which would otherwise enqueue a reconcile for every namespace in the cluster.
func TestReconcileArgoCD_imageUpdaterWatchNSMapper_EmptyPatterns(t *testing.T) {
	tests := []struct {
		name           string
		watchNamespace string
		triggerNS      string // namespace event that fires the mapper
		wantReconcile  bool   // whether a reconcile request is expected
	}{
		{
			name:           "trailing comma: only valid pattern matches, not everything",
			watchNamespace: "app-*,",
			triggerNS:      "app-frontend",
			wantReconcile:  true,
		},
		{
			name:           "trailing comma: non-matching namespace is still rejected",
			watchNamespace: "app-*,",
			triggerNS:      "other-ns",
			wantReconcile:  false,
		},
		{
			name:           "only commas: no valid patterns → no reconcile for any namespace",
			watchNamespace: ",,,",
			triggerNS:      "app-frontend",
			wantReconcile:  false,
		},
		{
			name:           "leading and trailing commas with valid pattern",
			watchNamespace: ",app-*,",
			triggerNS:      "app-frontend",
			wantReconcile:  true,
		},
		{
			name:           "leading and trailing commas: non-matching namespace is still rejected",
			watchNamespace: ",app-*,",
			triggerNS:      "other-ns",
			wantReconcile:  false,
		},
		{
			name:           "sole * with trailing comma is treated as cluster-scoped (skipped by mapper)",
			watchNamespace: "*,",
			triggerNS:      "any-namespace",
			wantReconcile:  false,
		},
		{
			name:           "* mixed with pattern is skipped by mapper (invalid config handled by reconciler)",
			watchNamespace: "*,team-a",
			triggerNS:      "team-a",
			wantReconcile:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			argocd := makeTestArgoCD()
			argocd.Name = "test-argocd"
			argocd.Namespace = "test-ns"
			argocd.Spec.ImageUpdater.Enabled = true
			argocd.Spec.ImageUpdater.Env = []corev1.EnvVar{
				{Name: "IMAGE_UPDATER_WATCH_NAMESPACES", Value: tt.watchNamespace},
			}
			argocd.ResourceVersion = ""

			resObjs := []client.Object{argocd}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, resObjs, []runtime.Object{})
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: tt.triggerNS}}
			got := r.imageUpdaterWatchNSMapper(context.TODO(), ns)

			if tt.wantReconcile {
				assert.Len(t, got, 1)
				assert.Equal(t, types.NamespacedName{Name: "test-argocd", Namespace: "test-ns"}, got[0].NamespacedName)
			} else {
				assert.Empty(t, got)
			}
		})
	}
}
