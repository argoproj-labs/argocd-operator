package gitserver

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

// ArgoCDWebhookURL builds the Argo CD git push webhook URL from a status host value.
func ArgoCDWebhookURL(host string, insecure bool) string {
	host = strings.TrimSpace(host)
	if idx := strings.Index(host, ","); idx >= 0 {
		host = strings.TrimSpace(host[:idx])
	}
	if host == "" {
		return ""
	}
	scheme := "https"
	if insecure {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s/api/webhook", scheme, host)
}

func (r *Repo) notifyArgoCDWebhook(webhookURL, host, branch, commitSHA string, changedFiles []string) error {
	Expect(webhookURL).NotTo(BeEmpty(), "webhook URL is required")

	ref := branch
	if !strings.HasPrefix(ref, "refs/") {
		ref = "refs/heads/" + branch
	}

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
			CloneURL:      r.GetRepoHttpURL(),
			DefaultBranch: "main",
			Private:       false,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	if host != "" {
		req.Host = host
		req.Header.Set("Host", host)
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
