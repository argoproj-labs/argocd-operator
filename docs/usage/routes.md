# Routes

The Argo CD Operator offers support for managing OpenShift Routes to access the Argo CD resources.

Once the operator is deployed and running, create a new ArgoCD custom resource.
The following example shows the minimal required to create a new ArgoCD
environment with the default configuration.

``` bash
oc create -f examples/argocd-basic.yaml
```

There will be several resources created.

``` bash
oc get pods
```
``` bash
NAME                                                     READY   STATUS    RESTARTS   AGE
example-argocd-application-controller-7c74b5855b-brn7s   1/1     Running   0          29s
example-argocd-dex-server-859bd5458c-78c8k               1/1     Running   0          29s
example-argocd-redis-6986d5fdbd-vzzjp                    1/1     Running   0          29s
example-argocd-repo-server-7bfc477c58-q7d8g              1/1     Running   0          29s
example-argocd-server-7d56c5bf4d-9wxz6                   1/1     Running   0          29s
argocd-operator-758dd86fb-qshll                          1/1     Running   0          51s
```

The ArgoCD Server should be available via an OpenShift Route.

``` bash
oc get routes
```

``` bash
NAME                        HOST/PORT                                               PATH   SERVICES                 PORT   TERMINATION     WILDCARD
example-argocd-server       example-argocd-server-argocd.apps.test.example.com              example-argocd-server    http   edge/Redirect   None
```

The Route is `example-argocd-server` in this example and should be available at the HOST/PORT value listed. The admin 
password is the name for the server Pod from above (`example-argocd-server-7d56c5bf4d-9wxz6` in this example).
