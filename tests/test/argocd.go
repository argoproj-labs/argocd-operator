package test

import (
	"time"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ArgoCDOpt func(*argoproj.ArgoCD)

func MakeTestArgoCD(a *argoproj.ArgoCD, opts ...ArgoCDOpt) *argoproj.ArgoCD {
	if a == nil {
		a = &argoproj.ArgoCD{
			ObjectMeta: metav1.ObjectMeta{
				Name:      TestArgoCDName,
				Namespace: TestNamespace,
			},
		}
	}
	for _, o := range opts {
		o(a)
	}
	return a
}

func ParallelismLimit(n int32) ArgoCDOpt {
	return func(a *argoproj.ArgoCD) {
		a.Spec.Controller.ParallelismLimit = n
	}
}

func LogFormat(f string) ArgoCDOpt {
	return func(a *argoproj.ArgoCD) {
		a.Spec.Controller.LogFormat = f
	}
}

func LogLevel(l string) ArgoCDOpt {
	return func(a *argoproj.ArgoCD) {
		a.Spec.Controller.LogLevel = l
	}
}

func ControllerProcessors(n int32) ArgoCDOpt {
	return func(a *argoproj.ArgoCD) {
		a.Spec.Controller.Processors.Status = n
	}
}

func OperationProcessors(n int32) ArgoCDOpt {
	return func(a *argoproj.ArgoCD) {
		a.Spec.Controller.Processors.Operation = n
	}
}

func AppSync(s int) ArgoCDOpt {
	return func(a *argoproj.ArgoCD) {
		a.Spec.Controller.AppSync = &metav1.Duration{Duration: time.Second * time.Duration(s)}
	}
}
