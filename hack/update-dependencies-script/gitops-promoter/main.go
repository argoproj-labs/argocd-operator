package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/argoproj-labs/argocd-operator/dependency-upgrade/utils"
)

const (
	GitOpsPromoterRepoURL        = "https://github.com/argoproj-labs/gitops-promoter"
	DefaultGitOpsPromoterVersion = "v0.34.0"
)

var targetGitOpsPromoterVersion string

func init() {
	version := os.Getenv("GITOPS_PROMOTER_VERSION")
	if version != "" {
		targetGitOpsPromoterVersion = version
	} else {
		targetGitOpsPromoterVersion = DefaultGitOpsPromoterVersion
	}
}

// TODO: Add debug statements
func main() {
	wd, err := os.Getwd()
	if err != nil {
		utils.ExitWithError(fmt.Errorf("unable to get working dir: %v", err))
		return
	}

	argocdOperatorRoot, err := filepath.Abs(wd + "/../../..")
	if err != nil {
		utils.ExitWithError(fmt.Errorf("unable to get absolute dir: %v", err))
		return
	}

	fmt.Println()
	fmt.Println("Cloning GitOps Promoter Repo")
	fmt.Println()

	// Clone GitOps Promoter into a temporary directory
	gitOpsPromoterRepoRoot, err := utils.CloneRepoIntoTempDir(GitOpsPromoterRepoURL, targetGitOpsPromoterVersion)
	if err != nil {
		utils.ExitWithError(fmt.Errorf("unable to checkout Argo CD: %v", err))
		return
	}

	// Sanity test that that we have the correct root path for argocd-operator repo
	if rootGoModExists, err := utils.FileExists(filepath.Join(argocdOperatorRoot, "go.mod")); err != nil || !rootGoModExists {
		utils.ExitWithError(fmt.Errorf("script should be run from 'hack/update-dependencies-script/gitops-promoter' directory: %v", err))
		return
	}

	fmt.Println()
	fmt.Println("Copying CRDs")
	fmt.Println()

	updateGitOpsPromoterCRDs(gitOpsPromoterRepoRoot, argocdOperatorRoot)
}

func updateGitOpsPromoterCRDs(promoterRepoRoot, operatorRoot string) {
	crdSourcePath := filepath.Join(promoterRepoRoot, "config", "crd", "bases")

	entries, err := os.ReadDir(crdSourcePath)
	if err != nil {
		utils.ExitWithError(fmt.Errorf("unable to list GitOps Promoter CRDs: %v", err))
		return
	}

	for _, entry := range entries {
		if !strings.Contains(entry.Name(), "argoproj.io") || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		srcFile := filepath.Join(crdSourcePath, entry.Name())
		destFile := filepath.Join(operatorRoot, "config", "crd", "bases", entry.Name())

		if err := utils.CopyFile(srcFile, destFile); err != nil {
			utils.ExitWithError(fmt.Errorf("unable to copy %s to %s: %v", srcFile, destFile, err))
			return
		}
	}
}
