// Package utils contains common shared functions between dependencies to update
package utils

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

func RunCommandListWithWorkDir(workingDir string, commands [][]string) error {
	for _, command := range commands {

		_, _, err := RunCommandWithWorkDir(workingDir, command...)
		if err != nil {
			return err
		}
	}
	return nil
}

func RunCommandWithWorkDir(workingDir string, cmdList ...string) (string, string, error) {
	fmt.Printf("%v:\n", cmdList)

	cmd := exec.Command(cmdList[0], cmdList[1:]...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Dir = workingDir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	stdoutStr := stdout.String()
	stderrStr := stderr.String()

	fmt.Println(stdoutStr, stderrStr)

	fmt.Println()

	return stdoutStr, stderrStr, err
}

func ExitWithError(err error) {
	fmt.Println("ERROR:", err)
	os.Exit(1)
}

func StripImagePrefix(line string) string {
	line = strings.TrimSpace(line)

	if !strings.HasPrefix(line, "image:") {
		ExitWithError(fmt.Errorf("unexpected image format on line: %s", line))
		return ""
	}

	return strings.TrimPrefix(line, "image: ")
}

func GrepForString(contents string, str string) []string {
	var res []string

	for _, line := range strings.Split(contents, "\n") {
		if strings.Contains(line, str) {
			res = append(res, line)
		}
	}

	return res
}

func RemoveDuplicateLines(in []string) []string {
	mapRes := map[string]any{}

	for _, inVal := range in {
		mapRes[inVal] = inVal
	}

	var res []string

	for k := range mapRes {
		res = append(res, k)
	}

	return res
}

func CopyFile(srcParam, dstParam string) (err error) {
	srcFile, err := os.Open(srcParam)
	if err != nil {
		return fmt.Errorf("could not open source file %s: %w", srcParam, err)
	}
	defer srcFile.Close()

	destFile, err := os.Create(dstParam)
	if err != nil {
		return fmt.Errorf("could not create destination file %s: %w", dstParam, err)
	}
	defer func() {
		if closeErr := destFile.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("could not close destination file %s: %w", dstParam, closeErr)
		}
	}()

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return fmt.Errorf("could not copy content from %s to %s: %w", srcParam, dstParam, err)
	}

	return nil
}

func FileExists(filename string) (bool, error) {
	info, err := os.Stat(filename)
	if err == nil {
		if info.IsDir() {
			return false, nil
		}
		return true, nil
	}

	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	return false, err
}

func GetFileContentsAsLines(filePath string) []string {
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		ExitWithError(fmt.Errorf("unable to get "+filePath+": %v", err))
		return nil
	}

	return strings.Split(string(bytes), "\n")
}

// CloneRepoIntoTempDir clones a specific repo into a temp directory
func CloneRepoIntoTempDir(repoURL, latestReleaseVersionTag string) (string, error) {
	url, err := url.Parse(repoURL)
	if err != nil {
		return "", err
	}
	cleanPath := strings.TrimRight(url.Path, "/")
	outputDir := path.Base(cleanPath)

	tmpDir, err := os.MkdirTemp("", "temp-repo")
	if err != nil {
		return "", err
	}

	if _, _, err := RunCommandWithWorkDir(tmpDir, "git", "clone",
		"--depth", "1",
		"--branch", latestReleaseVersionTag,
		"--single-branch",
		"--no-tags",
		repoURL,
		outputDir); err != nil {
		return "", err
	}

	newWorkDir := filepath.Join(tmpDir, outputDir)

	return newWorkDir, nil
}
