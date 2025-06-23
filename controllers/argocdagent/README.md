# Argo CD Agent

## Steps

1. Start operator
```
oc create ns argocd
make install
make run
```

2. Install Argo CD Agent CLI

3. Execute `controllers/argocdagent/create-agent-config.sh` script to configure cluster secret 

4. Create Argo CD resource having Principal component enabled
```
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: argocd
spec: 
  argoCDAgent:
    principal:
      enabled: true
```

5. Copy Principal LoadBalancer host and use it in Agent's ConfigMap `agent.server.address` to allow it to communicate with principal.

6. Execute command in managed cluster
`
argocd-agentctl pki issue agent agent-managed --agent-context vcluster-agent-managed --agent-namespace agent-managed --upsert

argocd-agentctl pki issue agent agent-autonomous --agent-context vcluster-agent-autonomous --agent-namespace argocd --upsert
`