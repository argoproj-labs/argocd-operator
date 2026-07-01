package gitserver

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

type gogsPushWebhookPayload struct {
	Ref        string                    `json:"ref"`
	Before     string                    `json:"before"`
	After      string                    `json:"after"`
	Commits    []gogsPushWebhookCommit   `json:"commits"`
	Repository gogsPushWebhookRepository `json:"repository"`
}

type gogsPushWebhookCommit struct {
	ID       string   `json:"id"`
	Message  string   `json:"message"`
	Added    []string `json:"added"`
	Removed  []string `json:"removed"`
	Modified []string `json:"modified"`
}

type gogsPushWebhookRepository struct {
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	HTMLURL       string `json:"html_url"`
	SSHURL        string `json:"ssh_url"`
	CloneURL      string `json:"clone_url"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
}

// NotifyArgoCDWebhook posts a push webhook to Argo CD for the current HEAD commit.
func (r *Repo) NotifyArgoCDWebhook(argoCD *argov1beta1api.ArgoCD, commit Commit) error {
	Expect(argoCD).NotTo(BeNil())
	Expect(argoCD.Status.Host).NotTo(BeEmpty())

	branch := commit.Branch
	if branch == "" {
		branch = "main"
	}

	commitSHA, err := r.git("rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("failed to resolve pushed commit: %w: %q", err, commitSHA)
	}

	ref := branch
	if !strings.HasPrefix(ref, "refs/") {
		ref = "refs/heads/" + branch
	}

	changedFiles := slices.Collect(maps.Keys(commit.Files))
	commitSHA = strings.TrimSpace(commitSHA)
	payload := gogsPushWebhookPayload{
		Ref:    ref,
		Before: strings.Repeat("0", 40),
		After:  commitSHA,
		Commits: []gogsPushWebhookCommit{{
			ID:       commitSHA,
			Message:  defaultCommitMessage,
			Modified: append([]string(nil), changedFiles...),
		}},
		Repository: gogsPushWebhookRepository{
			Name:          r.repoName,
			FullName:      fmt.Sprintf("%s/%s", r.server.httpUsername, r.repoName),
			HTMLURL:       fmt.Sprintf("https://%s:%d/%s/%s", r.server.domain, httpPort, r.server.httpUsername, r.repoName),
			SSHURL:        r.GetRepoSshURL(),
			CloneURL:      r.getRepoHttpURL(),
			DefaultBranch: "main",
			Private:       false,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	webhookURL := fmt.Sprintf("http://%s/api/webhook", argoCD.Status.Host)
	req, err := http.NewRequest(http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	if argoCD.Name != "" {
		req.Host = argoCD.Name
		req.Header.Set("Host", argoCD.Name)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gogs-Event", "push")
	req.Header.Set("X-Gitea-Event", "push")

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // #nosec G402 -- test clusters may use self-signed ingress certs
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("argo cd webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("argo cd webhook returned status %d", resp.StatusCode)
	}

	GinkgoWriter.Printf("notified Argo CD webhook %s for branch %s@%s\n", webhookURL, branch, commitSHA)
	return nil
}
