package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

func runCommandListWithWorkDir(workingDir string, commands [][]string) error {

	for _, command := range commands {

		_, _, err := runCommandWithWorkDir(workingDir, command...)
		if err != nil {
			return err
		}
	}
	return nil
}

func runCommandWithWorkDir(workingDir string, cmdList ...string) (string, string, error) {

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

func exitWithError(err error) {
	fmt.Println("ERROR:", err)
	os.Exit(1)
}

func stripImagePrefix(line string) string {
	line = strings.TrimSpace(line)

	if !strings.HasPrefix(line, "image:") {
		exitWithError(fmt.Errorf("unexpected image format on line: %s", line))
		return ""
	}

	return strings.TrimPrefix(line, "image: ")

}

func grepForString(contents string, str string) []string {

	var res []string

	for _, line := range strings.Split(contents, "\n") {

		if strings.Contains(line, str) {

			res = append(res, line)
		}

	}

	return res
}

func removeDuplicateLines(in []string) []string {

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

func copyFile(srcParam, dstParam string) (err error) {
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

func fileExists(filename string) (bool, error) {
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

func getFileContentsAsLines(filePath string) []string {
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		exitWithError(fmt.Errorf("unable to get "+filePath+": %v", err))
		return nil
	}

	return strings.Split(string(bytes), "\n")

}
