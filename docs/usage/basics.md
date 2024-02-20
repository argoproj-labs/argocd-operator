# Usage Basics

See the [ArgoCD Reference][argocd_reference] for the full list of properties and defaults to configure the Argo CD cluster.

The following example shows the most minimal valid manifest to create a new Argo CD cluster with the default configuration.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: basic
spec: {}
```

## Create

Create a new Argo CD cluster in the `argocd` namespace using the provided basic example.

```bash
kubectl create -n argocd -f examples/argocd-basic.yaml
```

There will be several Argo CD resources created that should be familiar to anyone who has deployed Argo CD.

```bash
kubectl get cm,secret,deploy -n argocd
```

Some unrelated items have been removed for clarity.

```bash
NAME                                  DATA   AGE
configmap/argocd-cm                   14     2m9s
configmap/argocd-rbac-cm              3      2m9s
configmap/argocd-ssh-known-hosts-cm   1      2m9s
configmap/argocd-tls-certs-cm         0      2m9s

NAME                                                   TYPE                                  DATA   AGE
secret/argocd-secret                                   Opaque                                5      2m9s
secret/example-argocd-ca                               kubernetes.io/tls                     2      2m9s
secret/example-argocd-cluster                          Opaque                                1      2m9s
secret/example-argocd-tls                              kubernetes.io/tls                     2      2m9s

NAME                                                    READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/example-argocd-application-controller   1/1     1            1           2m8s
deployment.apps/example-argocd-dex-server               1/1     1            1           2m8s
deployment.apps/example-argocd-redis                    1/1     1            1           2m8s
deployment.apps/example-argocd-repo-server              1/1     1            1           2m8s
deployment.apps/example-argocd-server                   1/1     1            1           2m8s
```

### ConfigMaps

There are several ConfigMaps that are used by Argo CD. The `argocd-server` component reads and writes to these
ConfigMaps based on user interaction with the web UI or via the `argocd` CLI. It is worth noting that the name
`argocd-cm` is hard-coded, thus limiting us to one Argo CD cluster per namespace to avoid conflicts.

```bash
NAME                                  DATA   AGE
configmap/argocd-cm                   14     33s
configmap/argocd-rbac-cm              3      33s
configmap/argocd-ssh-known-hosts-cm   1      33s
configmap/argocd-tls-certs-cm         0      33s
```

The operator will create these ConfigMaps for the cluster and set the initial values based on properties on the
`ArgoCD` custom resource.

### Secrets

There is a Secret that is used by Argo CD named `argocd-secret`. The `argocd-server` component reads this secret to
obtain the admin password for authentication. NOTE: Upon initial deployment, the initial password for the `admin` user is stored in the `argocd-cluster` secret instead.

This Secret is managed by the operator and should not be changed directly.

``` bash
NAME                                               TYPE                                  DATA   AGE
secret/argocd-secret                               Opaque                                5      33s
```

Several other Secrets are also managed by the operator.

``` bash
NAME                                               TYPE                                  DATA   AGE
secret/example-argocd-ca                           kubernetes.io/tls                     2      33s
secret/example-argocd-cluster                      Opaque                                1      33s
secret/example-argocd-tls                          kubernetes.io/tls                     2      33s
```

The cluster Secret contains the admin password for authenticating with Argo CD.

```bash
apiVersion: v1
data:
  admin.password: ...
kind: Secret
metadata:
  labels:
    app.kubernetes.io/name: example-argocd-cluster
    app.kubernetes.io/part-of: argocd
    example: basic
  name: example-argocd-cluster
  namespace: argocd
type: Opaque
```

The operator will watch for changes to the `admin.password` value. When a change is made the password is synchronized to
Argo CD automatically.

Fetch the admin password from the cluster Secret.

``` bash
kubectl -n argocd get secret example-argocd-cluster -o jsonpath='{.data.admin\.password}' | base64 -d
```

To change the admin password you'll need to modify the cluster secret like this:

```shell
$ kubectl -n argocd patch secret example-argocd-cluster \
  -p '{"stringData": {
    "admin.password": "newpassword2021"
  }}'
```

### Deployments

There are several Deployments that are managed by the operator for the different components that make up an Argo CD cluster.

``` bash
NAME                                                    READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/example-argocd-application-controller   1/1     1            1           2m8s
deployment.apps/example-argocd-dex-server               1/1     1            1           2m8s
deployment.apps/example-argocd-redis                    1/1     1            1           2m8s
deployment.apps/example-argocd-repo-server              1/1     1            1           2m8s
deployment.apps/example-argocd-server                   1/1     1            1           2m8s
```

The deployments are exposed via Services that can be used to access the Argo CD cluster.

### Services

The ArgoCD Server component should be available via a Service.

```bash
kubectl get svc -n argocd
```
```bash
NAME                            TYPE        CLUSTER-IP       EXTERNAL-IP   PORT(S)             AGE
example-argocd-dex-server       ClusterIP   10.105.36.155    <none>        5556/TCP,5557/TCP   2m28s
example-argocd-metrics          ClusterIP   10.102.88.192    <none>        8082/TCP            2m28s
example-argocd-redis            ClusterIP   10.101.29.123    <none>        6379/TCP            2m28s
example-argocd-repo-server      ClusterIP   10.103.229.32    <none>        8081/TCP,8084/TCP   2m28s
example-argocd-server           ClusterIP   10.100.186.222   <none>        80/TCP,443/TCP      2m28s
example-argocd-server-metrics   ClusterIP   10.100.185.144   <none>        8083/TCP            2m28s
argocd-operator-metrics         ClusterIP   10.97.124.166    <none>        8383/TCP,8686/TCP   23m
```

## Server API & UI

The Argo CD server component exposes the API and UI. The operator creates a Service to expose this component and
can be accessed through the various methods available in Kubernetes.

Follow the ArgoCD [Getting Started Guide](https://argoproj.github.io/argo-cd/getting_started/#creating-apps-via-ui) to
create a new application from the UI.

### Local Machine

In the most simple case, the Service port can be forwarded to the local machine.

```bash
kubectl -n argocd port-forward service/example-argocd-server 8443:443
```

The server UI should be available at [https://localhost:8443/](https://localhost:8443/).

### Ingress

See the [ingress][docs_ingress] documentation for steps to enable and use the Ingress support provided by the operator.

### OpenShift Route

See the [routes][docs_routes] documentation for steps to configure the Route support provided by the operator.

[docs_ingress]:./ingress.md
[docs_routes]:./routes.md
[argocd_reference]:../reference/argocd.md

### Default Permissions provided to Argo CD instance

By default Argo CD instance is provided the following permissions

* Argo CD instance is provided with ADMIN privileges for the namespace it is installed in. For instance, if an Argo CD instance is deployed in **foo** namespace, it will have **ADMIN privileges** to manage resources for that namespace.

* Argo CD is provided the following cluster scoped permissions because Argo CD requires cluster-wide read privileges on resources to function properly. (Please see [RBAC](https://argo-cd.readthedocs.io/en/stable/operator-manual/security/#cluster-rbac) section for more details.)

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  labels:
    app.kubernetes.io/managed-by: <argocd-instance-name>
    app.kubernetes.io/name: <argocd-instance-name>
    app.kubernetes.io/part-of: argocd
  name: <argocd-instance-name>-argocd-application-controller
  namespace: <argocd-instance-namespace>
rules:
- apiGroups:
  - '*'
  resources:
  - '*'
  verbs:
  - '*'
```

## Cluster Scoped Instance

The Argo CD instance created above can also be used to manage the cluster scoped resources by adding the namespace of the Argo CD instance to the `ARGOCD_CLUSTER_CONFIG_NAMESPACES` environment variable of subscription resource as shown below.

```yml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: argocd-operator
spec:
  config:
   env: 
    - name: ARGOCD_CLUSTER_CONFIG_NAMESPACES
      value: <list of namespaces of cluster-scoped Argo CD instances>
  channel: alpha
  name: argocd-operator
  source: argocd-catalog
  sourceNamespace: olm
```

### In-built permissions for cluster configuration

Argo CD is granted the following permissions using a cluster role when it is configured as cluster-scoped instance. **Argo CD is not granted cluster-admin**.

Please note that these permissions are in addition to the `admin` privileges that Argo CD has to the namespace in which it is installed.

```yaml
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: <argocd-instance-name>-<namespace>-argocd-application-controller
  labels:
    app.kubernetes.io/managed-by: <argocd-instance-namespace>
    app.kubernetes.io/name: <argocd-instance-namespace>
    app.kubernetes.io/part-of: argocd
rules:
  - verbs:
      - '*'
    apiGroups:
      - '*'
    resources:
      - '*'
```

### Additional permissions

Users can extend the permissions granted to Argo CD application controller by creating cluster roles with additional permissions and then a new cluster role binding to associate them to the service account.

For example, user can extend the permissions for an Argo CD instance to be able to list the secrets for all namespaces by creating the below resources.

#### Cluster Role

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  # "namespace" omitted since ClusterRoles are not namespaced
  name: secrets-cluster-role
rules:
- apiGroups: [""] #specifies core api groups
  resources: ["secrets"]
  verbs: ["*"]
```

#### Cluster Role Binding

```yaml
apiVersion: rbac.authorization.k8s.io/v1
# This cluster role binding allows Service Account to read secrets in any namespace.
kind: ClusterRoleBinding
metadata:
  name: read-secrets-global
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: secrets-cluster-role # Name of cluster role to be referenced
subjects:
- kind: ServiceAccount
  name: <argocd-instance-service-account-name>
  namespace: <argocd-instance-namespace>
```
