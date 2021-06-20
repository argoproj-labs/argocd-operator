module github.com/argoproj-labs/argocd-operator

go 1.16

require (
	cloud.google.com/go v0.56.0 // indirect
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.13.0
	github.com/openshift/api v3.9.0+incompatible
	google.golang.org/grpc v1.29.1 // indirect
	k8s.io/api v0.21.1
	k8s.io/apimachinery v0.21.1
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/cri-api v0.21.1
	sigs.k8s.io/controller-runtime v0.9.0
)

replace k8s.io/client-go => k8s.io/client-go v0.21.1 // Required by prometheus-operator
