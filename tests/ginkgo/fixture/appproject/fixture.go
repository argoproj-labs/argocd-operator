package appproject

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// ProjRef is a lightweight reference to an Argo CD AppProject.
type ProjRef struct {
	Name      string
	Namespace string
}

// projConfig holds configuration for creating an AppProject.
type projConfig struct {
	sourceRepos                []string
	destinations               [][2]string // [server, namespace]
	clusterResources           [][2]string // [group, kind]
	sourceNamespaces           []string
	destinationServiceAccounts []dsa
}

type dsa struct {
	Server, Namespace, Account string
}

// ProjOption configures AppProject creation.
type ProjOption func(*projConfig)

func WithSourceRepo(repo string) ProjOption {
	return func(c *projConfig) { c.sourceRepos = append(c.sourceRepos, repo) }
}

func WithDestination(server, namespace string) ProjOption {
	return func(c *projConfig) { c.destinations = append(c.destinations, [2]string{server, namespace}) }
}

func WithClusterResource(group, kind string) ProjOption {
	return func(c *projConfig) { c.clusterResources = append(c.clusterResources, [2]string{group, kind}) }
}

func WithSourceNamespace(ns string) ProjOption {
	return func(c *projConfig) { c.sourceNamespaces = append(c.sourceNamespaces, ns) }
}

func WithDestinationServiceAccount(server, namespace, account string) ProjOption {
	return func(c *projConfig) {
		c.destinationServiceAccounts = append(c.destinationServiceAccounts, dsa{
			Server: server, Namespace: namespace, Account: account,
		})
	}
}

// Create creates an Argo CD AppProject.
// Uses kubectl for projects with DestinationServiceAccounts (not supported by argocd CLI).
func Create(name, namespace string, opts ...ProjOption) *ProjRef {
	cfg := &projConfig{}
	for _, o := range opts {
		o(cfg)
	}

	ref := &ProjRef{Name: name, Namespace: namespace}

	if len(cfg.destinationServiceAccounts) > 0 {
		createViaKubectl(name, namespace, cfg)
	} else {
		createViaCLI(name, namespace, cfg)
	}

	return ref
}

func createViaCLI(name, namespace string, cfg *projConfig) {
	args := []string{"proj", "create", name, "--core", "-N", namespace}
	for _, repo := range cfg.sourceRepos {
		args = append(args, "--src", repo)
	}
	for _, dest := range cfg.destinations {
		args = append(args, "--dest", fmt.Sprintf("%s,%s", dest[0], dest[1]))
	}
	for _, cr := range cfg.clusterResources {
		args = append(args, "--allow-cluster-resource", fmt.Sprintf("%s/%s", cr[0], cr[1]))
	}

	output, err := runArgoCDCLI(args...)
	Expect(err).ToNot(HaveOccurred(), "argocd proj create failed: %s", output)

	// Source namespaces need separate commands
	for _, ns := range cfg.sourceNamespaces {
		out, err := runArgoCDCLI("proj", "add-source-namespace", name, ns, "--core", "-N", namespace)
		Expect(err).ToNot(HaveOccurred(), "argocd proj add-source-namespace failed: %s", out)
	}
}

func createViaKubectl(name, namespace string, cfg *projConfig) {
	spec := map[string]interface{}{}

	if len(cfg.sourceRepos) > 0 {
		spec["sourceRepos"] = cfg.sourceRepos
	}

	if len(cfg.destinations) > 0 {
		dests := make([]map[string]string, len(cfg.destinations))
		for i, d := range cfg.destinations {
			dests[i] = map[string]string{"server": d[0], "namespace": d[1]}
		}
		spec["destinations"] = dests
	}

	if len(cfg.clusterResources) > 0 {
		crs := make([]map[string]string, len(cfg.clusterResources))
		for i, cr := range cfg.clusterResources {
			crs[i] = map[string]string{"group": cr[0], "kind": cr[1]}
		}
		spec["clusterResourceWhitelist"] = crs
	}

	if len(cfg.destinationServiceAccounts) > 0 {
		dsas := make([]map[string]string, len(cfg.destinationServiceAccounts))
		for i, d := range cfg.destinationServiceAccounts {
			dsas[i] = map[string]string{
				"server":                d.Server,
				"namespace":             d.Namespace,
				"defaultServiceAccount": d.Account,
			}
		}
		spec["destinationServiceAccounts"] = dsas
	}

	if len(cfg.sourceNamespaces) > 0 {
		spec["sourceNamespaces"] = cfg.sourceNamespaces
	}

	resource := map[string]interface{}{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "AppProject",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": spec,
	}

	jsonBytes, err := json.Marshal(resource)
	Expect(err).ToNot(HaveOccurred())

	out, err := runKubectlWithStdin(string(jsonBytes), "apply", "-f", "-")
	Expect(err).ToNot(HaveOccurred(), "kubectl apply failed: %s", out)
}

// AddDestination adds a destination to an existing AppProject.
func AddDestination(ref *ProjRef, server, namespace string) {
	out, err := runArgoCDCLI("proj", "add-destination", ref.Name, server, namespace, "--core", "-N", ref.Namespace)
	Expect(err).ToNot(HaveOccurred(), "argocd proj add-destination failed: %s", out)
}

// AddSourceNamespace adds a source namespace to an existing AppProject.
func AddSourceNamespace(ref *ProjRef, ns string) {
	out, err := runArgoCDCLI("proj", "add-source-namespace", ref.Name, ns, "--core", "-N", ref.Namespace)
	Expect(err).ToNot(HaveOccurred(), "argocd proj add-source-namespace failed: %s", out)
}

// Ref creates a reference to an existing AppProject without creating it.
func Ref(name, namespace string) *ProjRef {
	return &ProjRef{Name: name, Namespace: namespace}
}

// --- Internal helpers ---

func runArgoCDCLI(args ...string) (string, error) {
	GinkgoWriter.Println("executing argocd", args)
	// #nosec G204 -- test code
	cmd := exec.Command("argocd", args...)
	output, err := cmd.CombinedOutput()
	GinkgoWriter.Println(string(output))
	return string(output), err
}

func runKubectlWithStdin(stdin string, args ...string) (string, error) {
	GinkgoWriter.Println("executing kubectl", args)
	// #nosec G204 -- test code
	cmd := exec.Command("kubectl", args...)
	cmd.Stdin = strings.NewReader(stdin)
	output, err := cmd.CombinedOutput()
	GinkgoWriter.Println(string(output))
	return string(output), err
}
