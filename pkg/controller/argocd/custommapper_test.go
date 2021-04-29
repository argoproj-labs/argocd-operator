package argocd

import (
	"reflect"
	"testing"

	"github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/common"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestReconcileArgoCD_clusterRoleBindingMapper(t *testing.T) {

	type fields struct {
		client client.Client
		scheme *runtime.Scheme
	}
	type args struct {
		o handler.MapObject
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
				o: handler.MapObject{
					Meta: &rbacv1.ClusterRoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								"argocds.argoproj.io/name": "foo",
								"foo/namespace":            "foo-ns",
							},
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
				o: handler.MapObject{
					Meta: &rbacv1.ClusterRoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								"argocds.argoproj.io/name":      "foo",
								"argocds.argoproj.io/namespace": "foo-ns",
							},
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
				o: handler.MapObject{
					Meta: &rbacv1.ClusterRoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								"foo/name":      "foo",
								"foo/namespace": "foo-ns",
							},
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
				client: tt.fields.client,
				scheme: tt.fields.scheme,
			}
			if got := r.clusterResourceMapper(tt.args.o); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReconcileArgoCD.clusterRoleBindingMapper() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReconcileArgoCD_tlsSecretMapper(t *testing.T) {
	argocd := &v1alpha1.ArgoCD{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd",
			Namespace: "argocd-operator",
			UID:       "abcd",
		},
	}

	t.Run("Map with proper ownerReference", func(t *testing.T) {
		service := &v1.Service{
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
		o := handler.MapObject{
			Meta:   secret,
			Object: secret,
		}
		objs := []runtime.Object{
			argocd,
			secret,
			service,
		}
		r := makeReconciler(t, argocd, objs...)
		want := []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      "argocd",
					Namespace: "argocd-operator",
				},
			},
		}
		got := r.tlsSecretMapper(o)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Reconciliation unsucessful: got: %v, want: %v", got, want)
		}
	})

	t.Run("Map with ownerReference on non-existing owner", func(t *testing.T) {
		service := &v1.Service{
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
		o := handler.MapObject{
			Meta:   secret,
			Object: secret,
		}
		objs := []runtime.Object{
			argocd,
			secret,
		}
		r := makeReconciler(t, argocd, objs...)
		want := []reconcile.Request{}
		got := r.tlsSecretMapper(o)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Reconciliation unsucessful: got: %v, want: %v", got, want)
		}
	})

	t.Run("Map with invalid owner", func(t *testing.T) {
		service := &v1.Service{
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
		o := handler.MapObject{
			Meta:   secret,
			Object: secret,
		}
		objs := []runtime.Object{
			argocd,
			secret,
			service,
		}
		r := makeReconciler(t, argocd, objs...)
		want := []reconcile.Request{}
		got := r.tlsSecretMapper(o)
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
		o := handler.MapObject{
			Meta:   secret,
			Object: secret,
		}
		objs := []runtime.Object{
			secret,
		}
		r := makeReconciler(t, argocd, objs...)
		want := []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      "argocd",
					Namespace: "argocd-operator",
				},
			},
		}
		got := r.tlsSecretMapper(o)
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
		o := handler.MapObject{
			Meta:   secret,
			Object: secret,
		}
		objs := []runtime.Object{
			secret,
		}
		r := makeReconciler(t, argocd, objs...)
		want := []reconcile.Request{}
		got := r.tlsSecretMapper(o)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Reconciliation unsucessful: got: %v, want: %v", got, want)
		}
	})

}
