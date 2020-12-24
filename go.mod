module github.com/argoproj-labs/argocd-operator

go 1.14

require (
	github.com/argoproj/argo-cd v1.5.8
	github.com/coreos/prometheus-operator v0.40.0
	github.com/go-openapi/spec v0.19.7
	github.com/google/go-cmp v0.4.0
	github.com/openshift/api v3.9.1-0.20190916204813-cdbe64fb0c91+incompatible
	github.com/operator-framework/operator-sdk v0.18.0
	github.com/sethvargo/go-password v0.2.0
	github.com/spf13/pflag v1.0.5
	gopkg.in/yaml.v2 v2.3.0
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.18.3
	k8s.io/apimachinery v0.18.3
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/kube-openapi v0.0.0-20200410145947-61e04a5be9a6
	sigs.k8s.io/controller-runtime v0.6.0
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by OLM
	k8s.io/client-go => k8s.io/client-go v0.18.2 // Required by prometheus-operator
)
