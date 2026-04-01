package appproject

import (
	"fmt"
	"os/exec"

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

// Create creates an Argo CD AppProject via the argocd CLI.
func Create(name, namespace string, opts ...ProjOption) *ProjRef {
	cfg := &projConfig{}
	for _, o := range opts {
		o(cfg)
	}

	Expect(cfg.session).ToNot(BeNil(), "WithSession is required for appproject.Create")

	ref := &ProjRef{Name: name, Namespace: namespace, session: cfg.session}

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
	for _, d := range cfg.destinationServiceAccounts {
		args = append(args, "--dest-service-accounts", fmt.Sprintf("%s,%s,%s", d.Server, d.Namespace, d.Account))
	}

	output, err := runArgoCDCLI(cfg.session, args...)
	Expect(err).ToNot(HaveOccurred(), "argocd proj create failed: %s", output)

	// Source namespaces need separate commands
	for _, ns := range cfg.sourceNamespaces {
		out, err := runArgoCDCLI(cfg.session, "proj", "add-source-namespace", name, ns)
		Expect(err).ToNot(HaveOccurred(), "argocd proj add-source-namespace failed: %s", out)
	}

	return ref
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
