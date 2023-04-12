# Plugins

See the [upstream documentation](https://argo-cd.readthedocs.io/en/stable/user-guide/config-management-plugins/#configure-plugin-via-sidecar) for more information.

Plugin sidecar containers can be added to the repo server using the `ArgoCD` custom resource.
If you want to specify the ConfigManagementPlugin manifest by specifying a config map,
the config map should be specified separately.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cmp-plugin
data:
  plugin.yaml: |
    apiVersion: argoproj.io/v1alpha1
    kind: ConfigManagementPlugin
    metadata:
      name: cmp-plugin
    spec:
      version: v1.0
      generate:
        command: [sh, -c, 'echo "{\"kind\": \"ConfigMap\", \"apiVersion\": \"v1\", \"metadata\": { \"name\": \"$ARGOCD_APP_NAME\", \"namespace\": \"$ARGOCD_APP_NAMESPACE\", \"annotations\": {\"Foo\": \"$FOO\", \"Bar\": \"baz\"}}}"']
      discover:
        find:
          command: [sh, -c, 'echo "FOUND"; exit 0']
      allowConcurrency: true
      lockRepo: true
---
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
spec:
  server:
    route:
      enabled: true
  repo:
    sidecarContainers:
      - name: cmp
        command: [/var/run/argocd/argocd-cmp-server]
        image: busybox
        securityContext:
          runAsNonRoot: true
          runAsUser: 999
        volumeMounts:
          - mountPath: /var/run/argocd
            name: var-files
          - mountPath: /home/argocd/cmp-server/plugins
            name: plugins
          - mountPath: /tmp
            name: tmp
          - mountPath: /home/argocd/cmp-server/config/plugin.yaml
            subPath: plugin.yaml
            name: cmp-plugin
    volumes:
      - configMap:
          name: cmp-plugin
        name: cmp-plugin
```
