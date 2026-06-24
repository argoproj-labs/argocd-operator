package gitserver

import (
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	. "github.com/onsi/gomega"
)

const defaultCommitMessage = "gitserver e2e commit"

type Repo struct {
	server   Server
	repoName string

	cloneDir   string
	sshKeyFile string
}

func (r Repo) GetRepoHttpURL() string {
	return fmt.Sprintf("https://%s:%d/%s/%s.git", r.server.domain, httpPort, r.server.httpUsername, r.repoName)
}

func (r Repo) GetRepoSshURL() string {
	return fmt.Sprintf("ssh://%s@%s:%d/%s/%s.git", giteaSSHLogin, r.server.domain, sshServicePort, r.server.httpUsername, r.repoName)
}

func (r *Repo) Clone() error {
	if r.sshKeyFile == "" {
		keyFile, err := r.sshPrivateKeyFile()
		if err != nil {
			return err
		}
		r.sshKeyFile = keyFile
	}

	parentDir, err := os.MkdirTemp("", "argocd-operator-gitserver-clone-*")
	if err != nil {
		return err
	}

	out, err := r.runGit(parentDir, "clone", r.GetRepoSshURL(), "repo")
	if err != nil {
		_ = os.RemoveAll(parentDir)
		return fmt.Errorf("failed to clone repo: %w: %s", err, out)
	}

	r.cloneDir = filepath.Join(parentDir, "repo")
	return nil
}

func (r *Repo) Git(args ...string) (out string, err error) {
	if r.cloneDir == "" {
		return "", fmt.Errorf("repository has not been cloned")
	}
	return r.runGit(r.cloneDir, args...)
}

// Fetch retrieves updates from the remote repository.
// branches can be empty, to fetch the default branch.
func (r *Repo) Fetch(branches ...string) error {
	if r.cloneDir == "" {
		return fmt.Errorf("repository has not been cloned")
	}

	args := append([]string{"fetch", "origin"}, branches...)
	if out, err := r.Git(args...); err != nil {
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
	if err := r.Fetch(branch); err != nil {
		return err
	}
	if out, err := r.Git("checkout", "-B", branch, "origin/"+branch); err != nil {
		return fmt.Errorf("failed to checkout branch %q: %w: %s", branch, err, out)
	}
	return nil
}

// ReadFile returns the contents of a file from the checked-out clone.
func (r *Repo) ReadFile(path string) (string, error) {
	if r.cloneDir == "" {
		return "", fmt.Errorf("repository has not been cloned")
	}
	data, err := os.ReadFile(filepath.Join(r.cloneDir, path))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (r *Repo) runGit(dir string, args ...string) (string, error) {
	if r.sshKeyFile == "" {
		return "", fmt.Errorf("ssh key file has not been created")
	}

	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(),
		"GIT_SSH_COMMAND=ssh -i "+r.sshKeyFile+
			" -o IdentitiesOnly=yes -o IdentityAgent=none"+
			" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null",
	)

	output, err := cmd.CombinedOutput()
	return string(output), err
}

func (r *Repo) sshPrivateKeyFile() (string, error) {
	f, err := os.CreateTemp("", "gitserver-ssh-key-")
	if err != nil {
		return "", err
	}

	if _, err := f.Write(r.server.GetSSHPrivateKey()); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}

	return f.Name(), nil
}

func (r *Repo) CommitAndPush(commit Commit) error {
	if r.cloneDir == "" {
		return fmt.Errorf("repository has not been cloned")
	}

	out, err := r.Git("status", "--porcelain")
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
		if out, err := r.Git("checkout", "-B", commit.Branch); err != nil {
			return fmt.Errorf("failed to checkout branch %q: %w: %s", commit.Branch, err, out)
		}
	}

	if out, err = r.Git("add", "-A"); err != nil {
		return fmt.Errorf("failed to add changes: %w: %s", err, out)
	}
	if out, err = r.Git("commit", "-m", defaultCommitMessage); err != nil {
		return fmt.Errorf("failed to commit changes: %w: %s", err, out)
	}

	pushArgs := []string{"push"}
	if commit.Branch != "" {
		pushArgs = append(pushArgs, "-u", "origin", commit.Branch)
	}
	if out, err = r.runGit(r.cloneDir, pushArgs...); err != nil {
		return fmt.Errorf("failed to push changes: %w: %s", err, out)
	}

	if commit.NotifyWebhookURL != "" {
		branch := commit.Branch
		if branch == "" {
			branch = "main"
		}
		sha, err := r.Git("rev-parse", "HEAD")
		if err != nil {
			return fmt.Errorf("failed to resolve pushed commit: %w: %q", err, sha)
		}

		changedFiles := slices.Collect(maps.Keys(commit.Files))
		if err := r.notifyArgoCDWebhook(commit.NotifyWebhookURL, commit.NotifyWebhookHost, branch, sha, changedFiles); err != nil {
			return err
		}
	}

	return nil
}

type Commit struct {
	Branch string
	Files  map[string]string
	// NotifyWebhookURL, when set, posts a Gogs-compatible push webhook to Argo CD after a successful push.
	NotifyWebhookURL string
	// NotifyWebhookHost is sent as the HTTP Host header when posting NotifyWebhookURL (required for ingress routing).
	NotifyWebhookHost string
}

func (c Commit) applyChange(repo *Repo) error {
	Expect(c.Files).NotTo(BeEmpty(), "commit must have at least one file")

	for path, content := range c.Files {
		fullPath := filepath.Join(repo.cloneDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return err
		}
	}
	return nil
}
