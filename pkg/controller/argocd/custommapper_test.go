package argocd

import (
	"reflect"
	"testing"

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
			if got := r.clusterRoleBindingMapper(tt.args.o); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReconcileArgoCD.clusterRoleBindingMapper() = %v, want %v", got, tt.want)
			}
		})
	}
}
