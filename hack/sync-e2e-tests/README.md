# sync-e2e-tests

An experimental utility that can be used to make it easier to keep argocd-operator and gitops-operator Ginkgo tests in sync.

This utility will strip the imports from a tree of go files. This allows for easy comparison of argocd-operator and gitops-operator E2E tests (since the imports are mainly all that differs between them).


## 1) Setup directories for comparison
```
mkdir /tmp/compare
cd /tmp/compare
git clone git@github.com:argoproj-labs/argocd-operator
cd argocd-operator
git checkout '(branch you want to compare)'

cd /tmp/compare
git clone git@github.com:redhat-developer/gitops-operator
cd gitops-operator
git checkout '(branch you want to compare)'
```

## 2) Strip imports from directories and commit 

```
go run . "/tmp/compare/argocd-operator"
go run . "/tmp/compare/gitops-operator"

cd /tmp/compare/argocd-operator
git add --all
git commit -m "hi"

cd /tmp/compare/gitops-operator
git add --all
git commit -m "hi"
```

## 3) Compare using meld
`meld /tmp/compare/gitops-operator/test/openshift/e2e/ginkgo /tmp/compare/argocd-operator/tests/ginkgo`

## 4) Generate patches for each repository

```
cd /tmp/compare/argocd-operator
git add --all
git commit -m "hi"
git diff HEAD~1 HEAD > /tmp/compare/argocd-operator/argocd-operator-patch.patch
```
# If the patch file contains the removed imports, you likely forgot to `git add`/`git commit` in previous steps

```
cd /tmp/compare/gitops-operator
git add --all
git commit -m "hi"
git diff HEAD~1 HEAD > /tmp/compare/gitops-operator/gitops-operator-patch.patch
```

## 5) Apply patch to parent repository

```
cd "(gitops-operator repository you will use to commit patch)"
git apply  /tmp/compare/gitops-operator/gitops-operator-patch.patch
```

```
cd "(argocd-operator repository you will use to commit patch)"
git apply  /tmp/compare/argocd-operator/argocd-operator-patch.patch
```

NOTE: If you see `patch does not apply` error, you can use `git apply --reject` instead, to apply only parts of the patch that succeed. Rejected patch segments will be stored with `.rej` file suffix, and can be manually applied.