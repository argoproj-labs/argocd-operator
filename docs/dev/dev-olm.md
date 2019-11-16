# Development Cycle for OLM

## Create


```bash
kubectl create namespace argocd
kubectl create -n olm -f deploy/catalog_source.yaml
kubectl create -n argocd -f deploy/operator_group.yaml
kubectl create -n argocd -f deploy/subscription.yaml
```

```bash
kubectl create -n argocd -f examples/argocd-ingress.yaml

kubectl create -n argocd -f examples/argocd-lb.yaml
```


## Cleanup

```bash
kubectl delete -n argocd -f examples/argocd-ingress.yaml

kubectl delete -n argocd -f examples/argocd-lb.yaml
```

```bash
kubectl delete -n argocd -f deploy/subscription.yaml
kubectl delete -n argocd -f deploy/operator_group.yaml
kubectl delete -n olm -f deploy/catalog_source.yaml
kubectl delete CustomResourceDefinition applications.argoproj.io 
kubectl delete CustomResourceDefinition appprojects.argoproj.io 
kubectl delete CustomResourceDefinition argocds.argoproj.io
kubectl delete namespace argocd
```