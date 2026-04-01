package appproject

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
)

// ProjRef is a lightweight reference to an Argo CD AppProject.
type ProjRef struct {
	Name      string
	Namespace string
	session   *argocdFixture.Session
}

// projConfig holds configuration for creating an AppProject.
type projConfig struct {
	sourceRepos                []string
	destinations               [][2]string // [server, namespace]
	clusterResources           [][2]string // [group, kind]
	sourceNamespaces           []string
	destinationServiceAccounts []dsa
	session                    *argocdFixture.Session
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

// WithSession sets the ArgoCD session for CLI authentication.
func WithSession(s *argocdFixture.Session) ProjOption {
	return func(c *projConfig) { c.session = s }
}

// Create creates an Argo CD AppProject.
// Uses kubectl for projects with DestinationServiceAccounts (not supported by argocd CLI).
func Create(name, namespace string, opts ...ProjOption) *ProjRef {
	cfg := &projConfig{}
	for _, o := range opts {
		o(cfg)
	}

	ref := &ProjRef{Name: name, Namespace: namespace, session: cfg.session}

	if len(cfg.destinationServiceAccounts) > 0 {
		createViaKubectl(name, namespace, cfg)
	} else {
		Expect(cfg.session).ToNot(BeNil(), "WithSession is required for appproject.Create via CLI")
		createViaCLI(name, namespace, cfg)
	}

	return ref
}

func createViaCLI(name, namespace string, cfg *projConfig) {
	args := []string{"proj", "create", name}
	for _, repo := range cfg.sourceRepos {
		args = append(args, "--src", repo)
	}
	for _, dest := range cfg.destinations {
		args = append(args, "--dest", fmt.Sprintf("%s,%s", dest[0], dest[1]))
	}
	for _, cr := range cfg.clusterResources {
		args = append(args, "--allow-cluster-resource", fmt.Sprintf("%s/%s", cr[0], cr[1]))
	}

	output, err := runArgoCDCLI(cfg.session, args...)
	Expect(err).ToNot(HaveOccurred(), "argocd proj create failed: %s", output)

	// Source namespaces need separate commands
	for _, ns := range cfg.sourceNamespaces {
		out, err := runArgoCDCLI(cfg.session, "proj", "add-source-namespace", name, ns)
		Expect(err).ToNot(HaveOccurred(), "argocd proj add-source-namespace failed: %s", out)
	}
}

func createViaKubectl(name, namespace string, cfg *projConfig) {
	spec := map[string]any{}

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

	resource := map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "AppProject",
		"metadata": map[string]any{
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
	Expect(ref.session).ToNot(BeNil(), "session is required for AddDestination")
	out, err := runArgoCDCLI(ref.session, "proj", "add-destination", ref.Name, server, namespace)
	Expect(err).ToNot(HaveOccurred(), "argocd proj add-destination failed: %s", out)
}

// AddSourceNamespace adds a source namespace to an existing AppProject.
func AddSourceNamespace(ref *ProjRef, ns string) {
	Expect(ref.session).ToNot(BeNil(), "session is required for AddSourceNamespace")
	out, err := runArgoCDCLI(ref.session, "proj", "add-source-namespace", ref.Name, ns)
	Expect(err).ToNot(HaveOccurred(), "argocd proj add-source-namespace failed: %s", out)
}

// Ref creates a reference to an existing AppProject without creating it.
// Session is optional — when provided, CLI operations (AddDestination, AddSourceNamespace) use it.
func Ref(name, namespace string, sessions ...*argocdFixture.Session) *ProjRef {
	var session *argocdFixture.Session
	if len(sessions) > 0 {
		session = sessions[0]
	}
	return &ProjRef{Name: name, Namespace: namespace, session: session}
}

// --- Internal helpers ---

func runArgoCDCLI(session *argocdFixture.Session, args ...string) (string, error) {
	allArgs := append([]string{"--server", session.Server, "--auth-token", session.AuthToken, "--insecure"}, args...)
	GinkgoWriter.Println("executing argocd", allArgs)
	// #nosec G204 -- test code
	cmd := exec.Command("argocd", allArgs...)
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
