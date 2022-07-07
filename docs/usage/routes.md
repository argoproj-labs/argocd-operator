# Routes

The Argo CD Operator offers support for managing OpenShift Routes to access the Argo CD resources.

Once the operator is deployed and running, create a new ArgoCD custom resource.
The following example shows the minimal required to create a new ArgoCD
environment with the default configuration.

``` bash
oc create -f examples/argocd-route.yaml
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
password is stored in the `argocd-cluster` secret in the installation namespace:

To get the password for the admin user:

```shell
$ kubectl get secret argocd-cluster -n argocd -ojsonpath='{.data.admin\.password}' | base64 --decode
```

## Setting TLS modes for routes

You can parameterize the route's TLS configuration by setting appropriate values in the `.spec.server.route.tls` field of the `ArgoCD` CR.

### TLS edge termination mode

In `edge` termination mode, the route controller terminates the TLS connection and proxies the requests
to Argo CD in plain text throughout the cluster.

The `edge` termination mode requires the Argo CD server to run in `insecure` mode, so it will accept
HTTP requests instead of TLS requests.

To set a route to `edge` mode, you can use the following configuration:

```yaml
spec:
  server:
    insecure: true
    route:
      enabled: true
      tls:
        termination: edge
        insecureEdgeTerminationPolicy: Redirect
```

Keep in mind that the connection will be unencrypted within your cluster.

### TLS passthrough mode

Passthrough will terminate TLS not on the route controller, but at the `argocd-server` service. This means,
that Argo CD will need to be configured with a valid TLS certificate, otherwise clients will issue
a warning upon trying to connect.

To set a route to `passthrough` mode, you can use the following configuration:

```yaml
spec:
  server:
    route:
      enabled: true
      tls:
        termination: passthrough
```

### TLS reencrypt mode

The `reencrypt` mode works a bit like the `edge` mode, in that TLS termination of the client
will happen at the route controller. However, unlike `edge` mode, the communication between
the route controller and the Argo CD server will be encrypted as well, so the Argo CD server
does not need to be set in `insecure` mode.

For this to work, the route controller needs to be able to validate the Argo CD server's TLS
certificate, otherwise the request will fail.

If you enable `reencrypt` mode in an OCP cluster, the Operator will request a valid TLS
certificate for the `argocd-server` service from OpenShift's Service CA, which is sufficient
for satisfying the validation constraints of the route controller. The Service CA will issue
this certificate to a secret named `argocd-server-tls` in the operand's namespace if it does
not yet exist.

When you later chose to switch back to another TLS termination policy, you should manually
delete the `argocd-server-tls` secret from the namespace after changing the mode.

To enable `reencrypt` mode, you can use the following configuration:

```yaml
spec:
  server:
    route:
      enabled: true
      tls:
        termination: reencrypt
        insecureEdgeTerminationPolicy: Redirect
```
### Host for Route in Argo CD Status

When setting up access to Argo CD via a Route, one can easily retrieve the hostname used for accessing the Argo CD installation through the ArgoCD Operand's `status` field. To expose the `host` field, run `kubectl edit argocd argocd` and then edit the Argo CD instance server to have route enabled as `true`, like so: 

```yaml
server:
    autoscale:
      enabled: false
    grpc:
      ingress:
        enabled: false
    ingress:
      enabled: false
    route:
      enabled: true
    service:
      type: ""
  tls:
    ca: {}
```
If a route is found, your hostname can now be accessed by inspecting your Argo CD instance. It will look like the following: 

```yaml
status:
  applicationController: Running
  dex: Running
  host: argocd-server-default.my-cluster-url.openshift.com
  phase: Available
  redis: Running
  repo: Running
  server: Running
  ssoConfig: Unknown
```

If the status of the Route is pending, this will affect the overall status of the Operand by making it `Pending` instead of `Available`. Once the Route is available, the status of the Operand should change to `Available`.

Note that Routes are specific to OpenShift clusters, so in non-OpenShift clusters enabling Route will yield no results.  
