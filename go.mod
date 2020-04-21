module github.com/argoproj-labs/argocd-operator

go 1.13

require (
	github.com/argoproj/argo-cd v1.5.2
	github.com/coreos/prometheus-operator v0.38.1
	github.com/go-openapi/spec v0.19.4
	github.com/openshift/api v3.9.1-0.20190916204813-cdbe64fb0c91+incompatible
	github.com/operator-framework/operator-sdk v0.17.0
	github.com/sethvargo/go-password v0.1.3
	github.com/spf13/pflag v1.0.5
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.17.4
	k8s.io/apimachinery v0.17.4
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/kube-openapi v0.0.0-20191107075043-30be4d16710a
	sigs.k8s.io/controller-runtime v0.5.2
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by OLM
	k8s.io/client-go => k8s.io/client-go v0.17.4 // Required by prometheus-operator
)
