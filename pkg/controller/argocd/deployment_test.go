package argocd

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
)

var (
	testNamespace  = "argocd"
	testArgoCDName = "argocd"
)

func Test_getArgoApplicationControllerComand(t *testing.T) {
	cmdTests := []struct {
		name string
		opts []argoCDOpt
		want []string
	}{
		{
			"defaults",
			[]argoCDOpt{},
			[]string{
				"argocd-application-controller",
				"--operation-processors",
				"10",
				"--redis",
				"argocd-redis:6379",
				"--repo-server",
				"argocd-repo-server:8081",
				"--status-processors",
				"20",
			},
		},
		{
			"configured status processors",
			[]argoCDOpt{controllerProcessors(30)},
			[]string{
				"argocd-application-controller",
				"--operation-processors",
				"10",
				"--redis",
				"argocd-redis:6379",
				"--repo-server",
				"argocd-repo-server:8081",
				"--status-processors",
				"30",
			},
		},
		{
			"configured operation processors",
			[]argoCDOpt{operationProcessors(15)},
			[]string{
				"argocd-application-controller",
				"--operation-processors",
				"15",
				"--redis",
				"argocd-redis:6379",
				"--repo-server",
				"argocd-repo-server:8081",
				"--status-processors",
				"20",
			},
		},
		{
			"configured appSync",
			[]argoCDOpt{appSync(600)},
			[]string{
				"argocd-application-controller",
				"--operation-processors",
				"10",
				"--redis",
				"argocd-redis:6379",
				"--repo-server",
				"argocd-repo-server:8081",
				"--status-processors",
				"20",
				"--app-resync",
				"600",
			},
		},
	}

	for _, tt := range cmdTests {
		cr := makeTestArgoCD(tt.opts...)
		cmd := getArgoApplicationControllerCommand(cr)

		if !reflect.DeepEqual(cmd, tt.want) {
			t.Fatalf("got %#v, want %#v", cmd, tt.want)
		}
	}
}

func controllerProcessors(n int32) argoCDOpt {
	return func(a *argoprojv1alpha1.ArgoCD) {
		a.Spec.Controller.Processors.Status = n
	}
}

func operationProcessors(n int32) argoCDOpt {
	return func(a *argoprojv1alpha1.ArgoCD) {
		a.Spec.Controller.Processors.Operation = n
	}
}

func appSync(n int64) argoCDOpt {
	return func(a *argoprojv1alpha1.ArgoCD) {
		a.Spec.Controller.AppSync = &n
	}
}

type argoCDOpt func(*argoprojv1alpha1.ArgoCD)

func makeTestArgoCD(opts ...argoCDOpt) *argoprojv1alpha1.ArgoCD {
	a := &argoprojv1alpha1.ArgoCD{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testArgoCDName,
			Namespace: testNamespace,
		},
	}
	for _, o := range opts {
		o(a)
	}
	return a
}
