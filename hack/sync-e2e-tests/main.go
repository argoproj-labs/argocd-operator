package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func main() {

	if len(os.Args) != 2 {
		fmt.Println("args: (path to modify)")
		return
	}

	pathToModify := os.Args[1]

	err := filepath.WalkDir(pathToModify, func(path string, d fs.DirEntry, err error) error {

		if strings.Contains(path, "..") {
			return fmt.Errorf("no path traversal")
		}

		// Safety check: Avoid walking any absolute paths which do not contain either gitops-operator or argocd-operator
		if !(strings.Contains(path, "gitops-operator") || strings.Contains(path, "argocd-operator")) {
			return fmt.Errorf("unrecognized path")
		}

		if !strings.HasPrefix(path, pathToModify) {
			return fmt.Errorf("missing prefix")
		}

		if d.IsDir() {
			return nil
		}

		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		fmt.Println(path)

		if err := stripImportsFromFile(path); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		fmt.Println(err)
		return
	}

}

func stripImportsFromFile(path string) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	var newContents string

	importSeen := false
	for _, line := range strings.Split(string(contents), "\n") {

		if strings.HasPrefix(line, "import (") {
			newContents += line + "\n"
			importSeen = true
			continue
		} else if importSeen && strings.HasPrefix(line, ")") {

			newContents += line + "\n"
			importSeen = false
			continue
		}

		if importSeen {

			line = ""

		} else {
			newContents += line + "\n"
		}
	}

	newContents = strings.TrimSuffix(newContents, "\n")

	if err := os.WriteFile(path, ([]byte)(newContents), 0600); err != nil {
		fmt.Println("unable to write", path, err)
		return err
	}

	return nil
}
