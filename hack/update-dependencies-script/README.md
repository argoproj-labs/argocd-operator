# update-dependencies-script

This is a simple go-based script that will upgrade the upstream dependencies in argocd-operator.

## To run this script:

In `(root)/Makefile`, modify `ARGO_CD_TARGET_VERSION` to target Argo CD version.

Example:
```
ARGO_CD_TARGET_VERSION ?= 3.1.8
```

Then run the script:
```
make update-dependencies
```

See `hack/update-dependencies-script/main.go` for list of dependencies that are updated.
