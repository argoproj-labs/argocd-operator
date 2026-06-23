# Ingress

The Argo CD Operator offers support for managing Ingress resources to access the Argo CD resources.

## Cluster

This guide builds on the [OLM Install Guide][install_olm] and assumes a Kubernetes cluster based on [minikube](https://minikube.sigs.k8s.io/).

### Ingress Controller

Ensure that the `ingress` addon is enabled for the minikube cluster.

```bash
minikube addons list -p argocd
```
```bash
- addon-manager: enabled
- dashboard: enabled
- default-storageclass: enabled
- efk: disabled
- freshpod: disabled
- gvisor: disabled
- heapster: disabled
- helm-tiller: disabled
- ingress: enabled
- ingress-dns: disabled
- logviewer: disabled
- metrics-server: disabled
- nvidia-driver-installer: disabled
- nvidia-gpu-device-plugin: disabled
- registry: disabled
- registry-creds: disabled
- storage-provisioner: enabled
- storage-provisioner-gluster: disabled
```

The addon is disabled by default, enable it if necessary.

```bash
minikube addons enable ingress -p argocd
```

Verify that the ingress Pod is running.

```bash
kubectl get pods -A
```
```bash
NAMESPACE              NAME                                                    READY   STATUS    RESTARTS   AGE
argocd                 example-argocd-application-controller-bdf64bc95-x9t7d   1/1     Running   0          92m
argocd                 example-argocd-dex-server-6b7b48d55d-xx4gn              1/1     Running   0          92m
argocd                 example-argocd-redis-7667b47db5-hpfzq                   1/1     Running   0          92m
argocd                 example-argocd-repo-server-c7f9889cd-555ld              1/1     Running   0          92m
argocd                 example-argocd-server-d468768b-l5wnf                    1/1     Running   0          92m
kube-system            coredns-5644d7b6d9-8rn4c                                1/1     Running   0          2d11h
kube-system            coredns-5644d7b6d9-ps44w                                1/1     Running   0          2d11h
kube-system            etcd-minikube                                           1/1     Running   0          2d11h
kube-system            kube-addon-manager-minikube                             1/1     Running   0          2d11h
kube-system            kube-apiserver-minikube                                 1/1     Running   0          2d11h
kube-system            kube-controller-manager-minikube                        1/1     Running   0          2d11h
kube-system            kube-proxy-8g2n4                                        1/1     Running   0          2d11h
kube-system            kube-scheduler-minikube                                 1/1     Running   0          2d11h
kube-system            nginx-ingress-controller-6fc5bcc8c9-vg26z               1/1     Running   0          9h
kube-system            storage-provisioner                                     1/1     Running   0          2d11h
kubernetes-dashboard   dashboard-metrics-scraper-76585494d8-2ksmc              1/1     Running   0          5h55m
kubernetes-dashboard   kubernetes-dashboard-57f4cb4545-w26nl                   1/1     Running   0          5h55m
olm                    catalog-operator-7dfcfcb46b-xxjdq                       1/1     Running   0          2d11h
olm                    olm-operator-76d446f94c-pn6kx                           1/1     Running   0          2d11h
olm                    operatorhubio-catalog-h4hc8                             1/1     Running   0          2d11h
olm                    packageserver-8478b89d9d-mtp56                          1/1     Running   0          45m
olm                    packageserver-8478b89d9d-vvswc                          1/1     Running   0          45m
```

In this example, the ingress controller is running in the `kube-system` namespace.

## ArgoCD Resource

Create an ArgoCD resource that enables ingress. Note that in this case we run the Argo CD server in insecure mode and 
terminate TLS at the Ingress controller. See `examples/argocd-ingress.yaml` for this example.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: ingress
spec:
  server:
    ingress:
      enabled: true
    insecure: true
```

Create the ArgoCD with Ingress support.

```bash
kubectl create -n argocd -f examples/argocd-ingress.yaml
```

By default, the Argo CD Operator creates two Ingress resources; one for the HTTP API/UI and the other for GRPC.

```bash
kubectl get ingress -n argocd
```
```bash
NAME                  HOSTS                 ADDRESS          PORTS     AGE
example-argocd        example-argocd        192.168.39.234   80, 443   68m
example-argocd-grpc   example-argocd-grpc   192.168.39.234   80, 443   68m
```

By default, the Host for each Ingress is based on the name of the ArgoCD resource. The Host can be overridden if needed.

## Access

In this example there are two hostnames that we will use to access the Argo CD cluster.

Add entries to the `/etc/hosts` file on the local machine, which is needed to access the services running locally on 
minikube.

```bash
echo "`minikube ip -p argocd` example-argocd example-argocd-grpc example-argocd-prometheus" | sudo tee -a /etc/hosts
```
```text
192.168.39.234 example-argocd example-argocd-grpc example-argocd-prometheus
```

### GRPC

The `argocd` client uses GRPC to communicate with the server. We can perform a login to verify access to the Argo CD 
server.

The default password for the admin user can be obtained using the below command.

```bash
kubectl get secret example-argocd-cluster -n argocd -ojsonpath='{.data.admin\.password}' | base64 -d ; echo
```

The `--insecure` flag is required because we are using the default self-signed certificate.

```bash
argocd login example-argocd-grpc --insecure --username admin 
```
```text
'admin' logged in successfully
Context 'example-argocd-grpc' updated
```

Create the example guestbook application based on the Argo CD [documentation][docs_argo].

```bash
argocd app create guestbook \
  --insecure \
  --repo https://github.com/argoproj/argocd-example-apps.git \
  --path guestbook \
  --sync-policy automated \
  --dest-server https://kubernetes.default.svc \
  --dest-namespace argocd
```
```text
application 'guestbook' created
```

Verify the application was created and has been synced.

```bash
argocd app list --insecure
```
```text
NAME       CLUSTER                         NAMESPACE  PROJECT  STATUS  HEALTH   SYNCPOLICY  CONDITIONS  REPO                                                 PATH       TARGET
guestbook  https://kubernetes.default.svc  argocd     default  Synced  Healthy  Auto        <none>      https://github.com/argoproj/argocd-example-apps.git  guestbook
```

Delete the application when finished.

```bash
argocd app delete guestbook --insecure
```

### UI

The server UI should be available at https://example-argocd/ and the default password for the admin user can be obtained using the below command.

```bash
kubectl get secret example-argocd-cluster -n argocd -ojsonpath='{.data.admin\.password}' | base64 -d ; echo
```

## Cleanup

```bash
kubectl delete -n argocd -f examples/argocd-ingress.yaml
```

[install_olm]:../install/olm.md
[docs_argo]:https://argoproj.github.io/argo-cd/getting_started/#creating-apps-via-cli

### Host for Ingress in Argo CD Status

When setting up access to Argo CD via an Ingress, one can easily retrieve hostnames used for accessing the Argo CD installation through the ArgoCD Operand's `status` field. To expose the `host` field, run `kubectl edit argocd argocd` and then edit the Argo CD instance server to have ingress enabled as `true`, like so: 

```yaml
server:
    autoscale:
      enabled: false
    grpc:
      ingress:
        enabled: false
    ingress:
      enabled: true
    route:
      enabled: false
    service:
      type: ""
  tls:
    ca: {}
```
If an ingress is found, hostname(s) of the ingress can now be accessed by inspecting your Argo CD instance. This data could be the hostname and/or the IP address(es), depending on what data is available. It will look like the following: 

```yaml
status:
  applicationController: Running
  dex: Running
  host: 172.24.0.7
  phase: Available
  redis: Running
  repo: Running
  server: Running
  ssoConfig: Unknown
```

If both Route and Ingress are enabled in the Argo CD spec and a route is available, the status for the Route will be prioritized over the Ingress's. In that case, the `host` for the Ingress is not shown in the `status`. 

Unlike with Routes, an Ingress does not go to pending status.  Hence, this will not affect the overall status of the Operand.
