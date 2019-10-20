# Helm Repositroy

## Create new GitHub Helm Repository project with GitHub Pages enabled 

* New Public Repository: helm-repo
* Respository Settings: GitHub Pages -> Source -> master branch

This will publish the new site at (example): `https://disposab1e.github.io/helm-repo/`



## Package Helm Chart and compose/test Helm Repository

Package: 
```bash
cd <clone of argocd-operator>/argocd-operator/helm

helm package argocd-operator

Successfully packaged chart and saved it to: <clone of argocd-operator>/argocd-operator/helm/argocd-operator-0.0.1.tgz
```

Compose:
```bash
mv argocd-operator-0.0.1.tgz helm-repo/

helm repo index helm-repo/ --url https://disposab1e.github.io/helm-repo 
```

Test:
```bash
helm serve --repo-path ./helm-repo/

Regenerating index. This may take a moment.
Now serving you on 127.0.0.1:8879
```
Point your Browser to: http://127.0.0.1:8879


## Clone Helm Repository project (from first step)

```bash
git clone https://github.com/disposab1e/helm-repo.git
```

## Publish Helm Repository

```bash
cd <clone of helm-repo>

mv <clone of argocd-operator>/argocd-operator/helm/helm-repo/index.yaml .

mv <clone of argocd-operator>/argocd-operator/helm/helm-repo/argocd-operator-0.0.1.tgz .

git add --all && git commit -m 'Argo CD Operator Helm Chart 0.0.1' && git push origin master
```
Point your Browser to: https://disposab1e.github.io/helm-repo
