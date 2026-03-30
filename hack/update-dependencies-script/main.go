package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {

	wd, err := os.Getwd()
	if err != nil {
		exitWithError(fmt.Errorf("unable to get working dir: %v", err))
		return
	}

	argocdOperatorRoot, err := filepath.Abs(wd + "/../..")
	if err != nil {
		exitWithError(fmt.Errorf("unable to get absolute dir: %v", err))
		return
	}

	// Version variable is in the form of 'v3.1.1', note the 'v' prefix
	targetArgoCDVersion := readTargetVersionFromMakefile(argocdOperatorRoot)

	fmt.Println()
	fmt.Println("Target Argo CD version from Makefile is", targetArgoCDVersion)
	fmt.Println()

	// Clone Argo CD into a temporary directory
	argoCDRepoRoot, err := cloneArgoCDRepoIntoTempDir(targetArgoCDVersion)
	if err != nil {
		exitWithError(fmt.Errorf("unable to checkout Argo CD: %v", err))
		return
	}

	fmt.Println("Using Argo CD temporary directory:", argoCDRepoRoot)

	// Sanity test that that we have the correct root path for argocd-operator repo
	if rootGoModExists, err := fileExists(filepath.Join(argocdOperatorRoot, "go.mod")); err != nil || !rootGoModExists {
		exitWithError(fmt.Errorf("script should be run from 'hack/update-dependencies-script' directory: %v", err))
		return
	}

	// update 'common/defaults.go': container images for dex, redis, redis HA, argo-cd, etc
	argocdContainerInfo, dexContainerInfo := upgradeCommonDefaultsGo(argoCDRepoRoot, argocdOperatorRoot)

	// update Argo CD crds
	updateArgoCDCRDs(argoCDRepoRoot, argocdOperatorRoot)

	// update go.mod and regenerate the manifests/bundle: acquire target argo cd version, and ensures replace block matches upstream
	updateGoModAndRegenBundle(targetArgoCDVersion, argocdOperatorRoot, argoCDRepoRoot)

	// update build/util/Dockerfile: update to reference target upstream container image
	updateBuildUtilDockerfile(argocdOperatorRoot, argocdContainerInfo)

	// update controllers/argocd/dex_test.go: test references a dex container image
	replaceDexImageReferenceInDexTest(argocdOperatorRoot, dexContainerInfo)

	fmt.Println()
	fmt.Println("Dependency update is complete:")
	fmt.Println("- You may wish to use a comparison tool to compare the 'manifests/install.yaml' file from the old and new versions of argo-cd, to verify no additional operator changes are required. ")
	fmt.Println("- You may wish to run 'make test' to verify the dependencies are in a consistent state.")
	fmt.Println()

}

func readTargetVersionFromMakefile(argocdOperatorRoot string) string {
	// Target version in Makefile looks like this:
	// ARGO_CD_TARGET_VERSION ?= 3.1.1

	fileToRead := filepath.Join(argocdOperatorRoot, "Makefile")

	bytes, err := os.ReadFile(fileToRead)
	if err != nil {
		exitWithError(fmt.Errorf("unable to read %s: %v", fileToRead, err))
		return ""
	}

	matches := grepForString(string(bytes), "ARGO_CD_TARGET_VERSION ?= ")

	if len(matches) != 1 {
		exitWithError(fmt.Errorf("unexpected number of matches for ARGO_CD_TARGET_VERSION: %v", matches))
		return ""
	}

	line := matches[0]

	if strings.Contains(line, "#") {
		exitWithError(fmt.Errorf("version line should not contain any comments"))
		return ""
	}

	setIndex := strings.Index(line, "?=")

	res := strings.TrimSpace(line[setIndex+2:])

	if !strings.HasPrefix(res, "v") {
		res = "v" + res
	}

	return res

}

func replaceDexImageReferenceInDexTest(argocdOperatorRoot string, dexContainerImage *processedContainerImage) {
	fileToUpdate := filepath.Join(argocdOperatorRoot, "controllers", "argocd", "dex_test.go")

	lines := getFileContentsAsLines(fileToUpdate)

	// Entry to replace looks like this:
	// Name:  "dex",
	// Image: "ghcr.io/dexidp/dex@sha256:d5f887574312f606c61e7e188cfb11ddb33ff3bf4bd9f06e6b1458efca75f604",

	newContent := ""

	match := false

	for idx := 0; idx < len(lines); idx++ {
		if strings.HasPrefix(lines[idx], "						Name:  \"dex\",") &&
			strings.HasPrefix(lines[idx+1], "						Image: \"ghcr.io/dexidp/dex@sha256:") {

			newContent += lines[idx] + "\n" // No modification of first line required
			newContent += "						Image: \"ghcr.io/dexidp/dex@" + dexContainerImage.sha256Digest + "\", // (" + dexContainerImage.version + ") NOTE: this value is modified by dependency update script\n"

			idx++ // Skip to the line after these 2
			match = true
		} else {
			newContent += lines[idx] + "\n"
		}
	}

	if !match {
		exitWithError(fmt.Errorf("unable to locate reference to dex image in dex_test.go"))
		return
	}

	newContent = strings.TrimSpace(newContent) + "\n"

	if err := os.WriteFile(fileToUpdate, []byte(newContent), 0600); err != nil {
		exitWithError(fmt.Errorf("unable to update %s: %v", fileToUpdate, err))
		return
	}

}

func updateBuildUtilDockerfile(argocdOperatorRoot string, argocdContainerImage *processedContainerImage) {

	fileToUpdate := filepath.Join(argocdOperatorRoot, "build", "util", "Dockerfile")

	lines := getFileContentsAsLines(fileToUpdate)

	// Entry to replace in Dockerfile looks like this:

	// # Argo CD v3.1.0-rc2
	// FROM quay.io/argoproj/argocd@sha256:dc4e00548b9e9fe31b6b2dca99b2278390faabd3610a04f4707dfddf66b5e90d as argocd

	newContent := ""

	match := false

	for idx := 0; idx < len(lines); idx++ {
		// Replace the argocd image references in the Dockerfile with new version
		if strings.HasPrefix(lines[idx], "# Argo CD v") && strings.HasPrefix(lines[idx+1], "FROM quay.io/argoproj/argocd@sha256") {
			newContent += "# Argo CD " + argocdContainerImage.version + "\n"
			newContent += "FROM quay.io/argoproj/argocd@" + argocdContainerImage.sha256Digest + " as argocd\n"
			idx++ // Skip to the line after these 2
			match = true
		} else {
			newContent += lines[idx] + "\n"
		}
	}

	if !match {
		exitWithError(fmt.Errorf("unable to locate reference to Argo CD container image in build/util/Dockerfile"))
		return
	}

	newContent = strings.TrimSpace(newContent) + "\n"

	if err := os.WriteFile(fileToUpdate, []byte(newContent), 0600); err != nil {
		exitWithError(fmt.Errorf("unable to update build/util/Dockerfile: %v", err))
		return
	}

}

// upgrade common/defaults.go
func upgradeCommonDefaultsGo(argoCDRepoRoot string, argocdOperatorRoot string) (*processedContainerImage, *processedContainerImage) {

	// Parse install YAML
	fileToParse := filepath.Join(argoCDRepoRoot, "manifests/ha/install.yaml")

	installYamlBytes, err := os.ReadFile(fileToParse)
	if err != nil {
		exitWithError(fmt.Errorf("unable to read Argo CD install yaml"))
		return nil, nil
	}

	installYAMLContents := string(installYamlBytes)

	targetDexImageLine := grepForString(installYAMLContents, "ghcr.io/dexidp/dex:")
	if len(targetDexImageLine) != 1 {
		exitWithError(fmt.Errorf("unexpected target dex image value: %v", targetDexImageLine))
		return nil, nil
	}
	targetDexImage := stripImagePrefix(targetDexImageLine[0])

	targetRedisImageLine := removeDuplicateLines(grepForString(installYAMLContents, "public.ecr.aws/docker/library/redis:"))
	if len(targetRedisImageLine) != 1 {
		exitWithError(fmt.Errorf("unexpected target redis image value: %v", targetRedisImageLine))
		return nil, nil
	}
	targetRedisImage := stripImagePrefix(targetRedisImageLine[0])

	targetHAProxyImageLine := removeDuplicateLines(grepForString(installYAMLContents, "public.ecr.aws/docker/library/haproxy:"))
	if len(targetHAProxyImageLine) != 1 {
		exitWithError(fmt.Errorf("unexpected target haproxy image value: %v", targetHAProxyImageLine))
		return nil, nil
	}
	targetHAProxyImage := stripImagePrefix(targetHAProxyImageLine[0])

	targetArgoCDImageLine := removeDuplicateLines(grepForString(installYAMLContents, "quay.io/argoproj/argocd:"))
	if len(targetArgoCDImageLine) != 1 {
		exitWithError(fmt.Errorf("unexpected target argo cd image value: %v", targetArgoCDImageLine))
		return nil, nil
	}
	targetArgoCDImage := stripImagePrefix(targetArgoCDImageLine[0])

	fmt.Println()
	fmt.Println("Found the following images in Argo CD manifests:")
	fmt.Println("-", targetArgoCDImage)
	fmt.Println("-", targetDexImage)
	fmt.Println("-", targetRedisImage)
	fmt.Println("-", targetHAProxyImage)

	dexContainerInfo := retrieveSHA256DigestUsingSkopeo(targetDexImage)
	redisContainerInfo := retrieveSHA256DigestUsingSkopeo(targetRedisImage)
	haProxyContainerInfo := retrieveSHA256DigestUsingSkopeo(targetHAProxyImage)
	argoCDContainerInfo := retrieveSHA256DigestUsingSkopeo(targetArgoCDImage)

	if err := replaceLineInCommonDefaultGo(argocdOperatorRoot, "ArgoCDDefaultArgoVersion", *argoCDContainerInfo); err != nil {
		exitWithError(fmt.Errorf("unable to replace dex version: %v", err))
		return nil, nil
	}

	if err := replaceLineInCommonDefaultGo(argocdOperatorRoot, "ArgoCDDefaultDexVersion", *dexContainerInfo); err != nil {
		exitWithError(fmt.Errorf("unable to replace dex version: %v", err))
		return nil, nil
	}

	if err := replaceLineInCommonDefaultGo(argocdOperatorRoot, "ArgoCDDefaultRedisVersionHA", *redisContainerInfo); err != nil {
		exitWithError(fmt.Errorf("unable to replace redis HA version: %v", err))
		return nil, nil
	}

	if err := replaceLineInCommonDefaultGo(argocdOperatorRoot, "ArgoCDDefaultRedisVersion", *redisContainerInfo); err != nil {
		exitWithError(fmt.Errorf("unable to replace redis version: %v", err))
		return nil, nil
	}

	if err := replaceLineInCommonDefaultGo(argocdOperatorRoot, "ArgoCDDefaultRedisHAProxyVersion", *haProxyContainerInfo); err != nil {
		exitWithError(fmt.Errorf("unable to replace redis HA proxy version: %v", err))
		return nil, nil
	}

	return argoCDContainerInfo, dexContainerInfo
}

func updateArgoCDCRDs(argoCDRepoRoot string, argocdOperatorRoot string) {

	argoCDCRDSourcePath := filepath.Join(argoCDRepoRoot, "manifests", "crds")

	entries, err := os.ReadDir(argoCDCRDSourcePath)
	if err != nil {
		exitWithError(fmt.Errorf("unable to list Argo CD CRDS: %v", err))
		return
	}

	var count int
	for _, entry := range entries {
		fname := entry.Name()
		if entry.IsDir() || fname == "kustomization.yaml" {
			continue
		}

		if strings.HasSuffix(fname, ".yaml") {
			count++
		}
	}

	filesToCopy := map[string]string{
		// argo cd repo YAML filename -> argocd-operator YAML filename
		"application-crd.yaml":    "argoproj.io_applications.yaml",
		"applicationset-crd.yaml": "argoproj.io_applicationsets.yaml",
		"appproject-crd.yaml":     "argoproj.io_appprojects.yaml",
	}

	// Sanity test: The CRDs found in Argo CD directory should match the values in the map above
	if len(filesToCopy) != count {
		exitWithError(fmt.Errorf("unexpected number of YAML files found in Argo CD CRD directory '%s': %d", argoCDCRDSourcePath, count))
		return
	}

	for k, v := range filesToCopy {
		srcFile := filepath.Join(argoCDCRDSourcePath, k)
		destFile := filepath.Join(argocdOperatorRoot, "config", "crd", "bases", v)

		if err := copyFile(srcFile, destFile); err != nil {
			exitWithError(fmt.Errorf("unable to copy %s to %s: %v", srcFile, destFile, err))
			return
		}

	}

}

// readReplaceBlockFromGoMod will read the following block from a go.mod:
//
// module github.com/argoproj-labs/argocd-operator
//
// go 1.24.6
// require (
//
//	(...)
//
// )
// replace (
//
//	// <=== The values from inside here are returned
//
// )
//
// This function expects only one replace block to exist, and will fail if 0 or >1 are found.
func readReplaceBlockFromGoMod(pathToGoMod string) ([]string, int, int) {
	lines := getFileContentsAsLines(pathToGoMod)

	replaceBlockStart := -1
	replaceBlockEnd := -1
	for x := 0; x < len(lines); x++ {

		line := lines[x]
		if strings.HasPrefix(line, "replace (") {

			if replaceBlockStart != -1 {
				exitWithError(fmt.Errorf("multiple replace blocks detected in argocd go.mod"))
				return nil, replaceBlockStart, replaceBlockEnd
			}

			replaceBlockStart = x + 1
		}

		if replaceBlockStart != -1 && strings.HasPrefix(line, ")") {
			replaceBlockEnd = x - 1
		}
	}

	if replaceBlockStart == -1 {
		exitWithError(fmt.Errorf("replace block start not found"))
		return nil, replaceBlockStart, replaceBlockEnd
	}

	if replaceBlockEnd == -1 {
		exitWithError(fmt.Errorf("replace block end not found"))
		return nil, replaceBlockStart, replaceBlockEnd
	}

	return lines, replaceBlockStart, replaceBlockEnd
}

// copyGoModReplaceBlockFromArgoCDToArgoCDOperator will look at the replace ( ... ) block from argocd's go.mod, and copy it, as is, into the replace block of argocd-operator go.mod.
// - This code assumes that only 1 replace ( ... ) block exists in both argocd go.mod and argocd-operator go.mod
// - It also assumes that no other non-argo-cd values have been inserted into argocd-operator go.mod
func copyGoModReplaceBlockFromArgoCDToArgoCDOperator(argoCDRepoRoot string, argocdOperatorRoot string, targetArgoCDVersion string) {

	argoCDRepoRootGoModPath := filepath.Join(argoCDRepoRoot, "go.mod")

	replaceBlockFromArgoCDLines, replaceBlockStart, replaceBlockEnd := readReplaceBlockFromGoMod(argoCDRepoRootGoModPath)
	replaceBlockFromArgoCD := replaceBlockFromArgoCDLines[replaceBlockStart : replaceBlockEnd+1]

	argocdOperatorGoModPath := filepath.Join(argocdOperatorRoot, "go.mod")

	argocdOperatorLines, argocdOperatorReplaceStart, argocdOperatorReplaceEnd := readReplaceBlockFromGoMod(argocdOperatorGoModPath)

	newArgoCDOperatorGoModFileContents := ""

	for x, line := range argocdOperatorLines {
		if x == argocdOperatorReplaceStart {

			newArgoCDOperatorGoModFileContents += "\t// This replace block is from Argo CD " + targetArgoCDVersion + " go.mod\n"

			// inject the replace block from argocd's go.mod
			for _, replaceBlockFromArgoCDLine := range replaceBlockFromArgoCD {
				newArgoCDOperatorGoModFileContents += replaceBlockFromArgoCDLine + "\n"
			}

		} else if x >= argocdOperatorReplaceStart && x <= argocdOperatorReplaceEnd {
			// skip existing replace block in argocd-operator's go.mod
			continue
		} else {
			newArgoCDOperatorGoModFileContents += line + "\n"
		}
	}
	if err := os.WriteFile(argocdOperatorGoModPath, ([]byte)(newArgoCDOperatorGoModFileContents), 0600); err != nil {
		exitWithError(fmt.Errorf("unable to write to file: %s %v", argocdOperatorGoModPath, err))
		return
	}
}

// updateGoModAndRegenBundle ensures that argocd-operator go.mod is update to date with latest from upstream argocd-operator
func updateGoModAndRegenBundle(targetArgoCDVersion string, argocdOperatorRoot string, argoCDRepoRoot string) {
	err := runCommandListWithWorkDir(argocdOperatorRoot,
		[][]string{
			{"go", "get", "github.com/argoproj/argo-cd/v3@" + targetArgoCDVersion},
		})
	if err != nil {
		exitWithError(fmt.Errorf("unable to update argocd-operator go.mod"))
		return
	}

	copyGoModReplaceBlockFromArgoCDToArgoCDOperator(argoCDRepoRoot, argocdOperatorRoot, targetArgoCDVersion)

	err = runCommandListWithWorkDir(argocdOperatorRoot,
		[][]string{
			{"go", "mod", "tidy"},
			{"rm", "-f", argocdOperatorRoot + "/bin/controller-gen"}, // Erase the controller-gen binary to ensure it is re-downloaded to the correct version
			{"make", "generate", "manifests"},
			{"make", "bundle"},
			{"make", "fmt"},
		})
	if err != nil {
		exitWithError(fmt.Errorf("unable to update argocd-operator go.mod"))
	}

}

// replaceLineInCommonDefaultGo updates a line in 'common/defaults.go' beginning with 'variableToReplace' with a different container image digest and version comment
//
// Example:
//   - From:
//     ArgoCDDefaultArgoVersion = "sha256:dc4e00548b9e9fe31b6b2dca99b2278390faabd3610a04f4707dfddf66b5e90d" // v3.1.0-rc2
//   - To:
//     ArgoCDDefaultArgoVersion = "sha256:a36ab0c0860c77159c16e04c7e786e7a282f04889ba9318052f0b8897d6d2040" // v3.1.1
func replaceLineInCommonDefaultGo(pathToArgoCDGitRepoRoot string, variableToReplace string, toReplace processedContainerImage) error {

	path := filepath.Join(pathToArgoCDGitRepoRoot, "common/defaults.go")

	lines := getFileContentsAsLines(path)

	var res string

	var match bool
	for _, line := range lines {

		if strings.Contains(line, "\t"+variableToReplace+" ") {
			match = true

			res += "\t" + variableToReplace + " = \"" + toReplace.sha256Digest + "\" // " + toReplace.version + "\n"

		} else {
			res += line + "\n"
		}

	}

	if !match {
		return fmt.Errorf("no match found for '%s'", variableToReplace)
	}

	res = strings.TrimSpace(res) + "\n"

	if err := os.WriteFile(path, []byte(res), 0600); err != nil {
		return err
	}

	return nil

}

// cloneArgoCDRepoIntoTempDir clones a specific argo-cd version into a temp dir
func cloneArgoCDRepoIntoTempDir(latestReleaseVersionTag string) (string, error) {

	tmpDir, err := os.MkdirTemp("", "argo-cd-src")
	if err != nil {
		return "", err
	}

	if _, _, err := runCommandWithWorkDir(tmpDir, "git", "clone", "https://github.com/argoproj/argo-cd"); err != nil {
		return "", err
	}

	newWorkDir := filepath.Join(tmpDir, "argo-cd")

	commands := [][]string{
		{"git", "checkout", latestReleaseVersionTag},
	}

	if err := runCommandListWithWorkDir(newWorkDir, commands); err != nil {
		return "", err
	}

	return newWorkDir, nil
}

// retrieveSHA256DigestUsingSkopeo determines the SHA256 digest value for a given container image
func retrieveSHA256DigestUsingSkopeo(url string) *processedContainerImage {

	stdout, _, err := runCommandWithWorkDir("", "skopeo", "inspect", "--no-tags", "docker://"+url)
	if err != nil {
		exitWithError(fmt.Errorf("unexpected skopeo error: %v", err))
		return nil
	}

	// Skopeo inspect output looks like this:
	// 	{
	//     "Name": "quay.io/argoproj/argocd",
	//     "Digest": "sha256:a36ab0c0860c77159c16e04c7e786e7a282f04889ba9318052f0b8897d6d2040",
	//     "RepoTags": [],
	//     "Created": "2025-08-25T15:56:23.164856076Z",
	// 	# (...)
	//     "Architecture": "amd64",
	//     "Os": "linux",
	// }

	// Extract Digest value from JSON
	var jsonMap map[string]any
	if err := json.Unmarshal([]byte(stdout), &jsonMap); err != nil {
		exitWithError(fmt.Errorf("unexpected unmarshal error: %v", err))
		return nil
	}

	digestVal, ok := jsonMap["Digest"]
	if !ok || digestVal == nil {
		exitWithError(fmt.Errorf("unable to extract digest val for %s: key missing or nil", url))
		return nil
	}

	sha256Digest, ok := digestVal.(string)
	if !ok || sha256Digest == "" {
		exitWithError(fmt.Errorf("unable to extract digest val for %s: not a valid string", url))
		return nil
	}

	return &processedContainerImage{
		version:      url[strings.Index(url, ":")+1:],
		sha256Digest: sha256Digest,
	}
}

// processedContainerImage is the return values for processedContainerImage
type processedContainerImage struct {
	version      string // version string portion of container image URL, e.g. 'v3.1.1'
	sha256Digest string // 'sha256:(...)' digest value of container image from skopeo. string includes 'sha256:' prefix
}
