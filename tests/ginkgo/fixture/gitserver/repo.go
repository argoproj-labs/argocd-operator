package gitserver

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/gomega"
)

type Transport string

const (
	TransportSSH   Transport = "ssh"
	TransportHTTPS Transport = "https"
)

const defaultCommitMessage = "gitserver e2e commit"

type Repo struct {
	server   *Server
	repoName string

	cloneDir  *os.Root
	transport Transport
}

// GetRepoHttpURL returns the HTTPS clone URL reachable from inside the cluster.
// Argo CD components use this URL; it matches the Git server TLS certificate SAN.
func (r Repo) GetRepoHttpURL() string {
	return fmt.Sprintf("https://%s:%d/%s/%s.git", r.server.clusterDomain, httpPort, r.server.httpUsername, r.repoName)
}

func (r Repo) getRepoHttpURLWithCredentials() string {
	repoURL := &url.URL{
		Scheme: "https",
		User:   url.UserPassword(r.server.httpUsername, r.server.httpPassword),
		Host:   fmt.Sprintf("%s:%d", r.server.domain, httpPort),
		Path:   fmt.Sprintf("/%s/%s.git", r.server.httpUsername, r.repoName),
	}
	return repoURL.String()
}

func (r Repo) GetRepoSshURL() string {
	return fmt.Sprintf("ssh://%s@%s:%d/%s/%s.git", giteaSSHLogin, r.server.domain, sshServicePort, r.server.httpUsername, r.repoName)
}

func (r *Repo) Clone(t Transport) (cleanup func(), err error) {
	r.transport = t

	if t == TransportSSH {
		if _, err := r.server.getSSHKeyFile(); err != nil {
			return nil, err
		}
	}

	fsDir, err := os.MkdirTemp("", "operator-gitserver-clone-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}

	r.cloneDir, err = os.OpenRoot(fsDir)
	if err != nil {
		_ = os.RemoveAll(fsDir)
		return nil, fmt.Errorf("failed to open root: %w", err)
	}
	cleanup = func() {
		_ = r.cloneDir.Close()
		r.cloneDir = nil
		_ = os.RemoveAll(fsDir)
	}

	cloneURL := r.GetRepoSshURL()
	if t == TransportHTTPS {
		cloneURL = r.getRepoHttpURLWithCredentials()
	}

	out, err := r.git("clone", cloneURL, ".")
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to clone repo: %w: %s", err, out)
	}

	if out, err := r.git("config", "user.name", "E2E test"); err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to set git user.name: %w: %s", err, out)
	}
	if out, err := r.git("config", "user.email", "e2e-test@example.com"); err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to set git user.email: %w: %s", err, out)
	}

	return cleanup, nil
}

func (r *Repo) fetch(branches ...string) error {
	if r.cloneDir == nil {
		return fmt.Errorf("repository has not been cloned")
	}

	args := append([]string{"fetch", "origin"}, branches...)
	if out, err := r.git(args...); err != nil {
		if len(branches) == 0 {
			return fmt.Errorf("failed to fetch from origin: %w: %s", err, out)
		}
		return fmt.Errorf("failed to fetch branches %v from origin: %w: %q", branches, err, out)
	}
	return nil
}

// CheckoutBranch fetches and checks out a remote branch.
func (r *Repo) CheckoutBranch(branch string) error {
	if branch == "" {
		return fmt.Errorf("branch is required")
	}
	if err := r.fetch(branch); err != nil {
		return err
	}
	if out, err := r.git("checkout", "-B", branch, "origin/"+branch); err != nil {
		return fmt.Errorf("failed to checkout branch %q: %w: %s", branch, err, out)
	}
	return nil
}

// ReadFile returns the contents of a file from the checked-out clone.
func (r *Repo) ReadFile(path string) (string, error) {
	data, err := r.cloneDir.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (r *Repo) git(args ...string) (string, error) {
	if r.cloneDir == nil {
		return "", fmt.Errorf("repository has not been cloned")
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = r.cloneDir.Name()
	if r.transport == TransportHTTPS {
		cmd.Env = append(os.Environ(), "GIT_SSL_NO_VERIFY=true")
	} else {
		sshKeyFile, err := r.server.getSSHKeyFile()
		if err != nil {
			return "", err
		}
		cmd.Env = append(os.Environ(),
			"GIT_SSH_COMMAND=ssh -i "+sshKeyFile+
				" -o IdentitiesOnly=yes -o IdentityAgent=none"+
				" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null",
		)
	}

	output, err := cmd.CombinedOutput()
	return string(output), err
}

func (r *Repo) CommitAndPush(commit Commit) error {
	if r.cloneDir == nil {
		return fmt.Errorf("repository has not been cloned")
	}

	out, err := r.git("status", "--porcelain")
	if err != nil {
		return fmt.Errorf("failed to check repository status: %w: %s", err, out)
	}
	if strings.TrimSpace(out) != "" {
		return fmt.Errorf("repository has uncommitted changes: %s", strings.TrimSpace(out))
	}

	if err := commit.applyChange(r); err != nil {
		return err
	}

	if commit.Branch != "" {
		if out, err := r.git("checkout", "-B", commit.Branch); err != nil {
			return fmt.Errorf("failed to checkout branch %q: %w: %s", commit.Branch, err, out)
		}
	}

	if out, err = r.git("add", "-A"); err != nil {
		return fmt.Errorf("failed to add changes: %w: %s", err, out)
	}
	if out, err = r.git("commit", "-m", defaultCommitMessage); err != nil {
		return fmt.Errorf("failed to commit changes: %w: %s", err, out)
	}

	pushArgs := []string{"push"}
	if commit.Branch != "" {
		pushArgs = append(pushArgs, "-u", "origin", commit.Branch)
	}
	if out, err = r.git(pushArgs...); err != nil {
		return fmt.Errorf("failed to push changes: %w: %s", err, out)
	}

	return nil
}

type Commit struct {
	Branch string
	Files  map[string]string
}

func (c Commit) applyChange(repo *Repo) error {
	Expect(c.Files).NotTo(BeEmpty(), "commit must have at least one file")

	for path, content := range c.Files {
		if err := repo.cloneDir.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return err
		}
		if err := repo.cloneDir.WriteFile(path, []byte(content), 0o600); err != nil {
			return err
		}
	}
	return nil
}
