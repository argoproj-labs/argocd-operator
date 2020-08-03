module github.com/argoproj-labs/argocd-operator

go 1.13

require (
	github.com/coreos/prometheus-operator v0.41.0
	github.com/go-logr/logr v0.1.0
	github.com/onsi/ginkgo v1.12.0
	github.com/onsi/gomega v1.9.0
	github.com/openshift/api v0.0.0-20200116145750-0e2ff1e215dd
	github.com/sethvargo/go-password v0.2.0
	golang.org/x/crypto v0.0.0-20200422194213-44a606286825
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.18.3
	k8s.io/apimachinery v0.18.3
	k8s.io/client-go v0.18.3
	sigs.k8s.io/controller-runtime v0.6.0
)
