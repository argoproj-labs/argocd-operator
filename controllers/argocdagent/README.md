# Argo CD Agent Setup Guide

## Overview

This guide provides step-by-step instructions for setting up and configuring the Argo CD Agent in a dev environment.

## Prerequisites

- The `argocd-agentctl` CLI tool installed

## Steps for Principal Set up

### Step 1: Create Required Namespaces

Create a namespace for principal in control plane cluster.
```bash
oc create ns argocd
```
Create a namespaces for agents in control plane cluster. This should be same as [agent namespace](https://github.com/argoproj-labs/argocd-agent/blob/main/install/kubernetes/agent/agent-params-cm.yaml#L50) in workload clusters.

```bash
oc create ns agent-autonomous
oc create ns agent-managed
```

### Step 2: Start the Operator

Install the required CRDs and start the Argo CD operator development mode.

```bash
make install
export ARGOCD_CLUSTER_CONFIG_NAMESPACES="argocd"
make run
```

### Step 3: Deploy Argo CD Instance with Principal

Use below yaml content to create Argo CD instance in `argocd` namespace.

```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: argocd
spec:
  controller:
    enabled: false
  argoCDAgent:
    principal:
      enabled: true
      allowed-namespaces: "*"
      jwt-allow-generate: true
      auth: "mtls:CN=([^,]+)"
      logLevel: "trace"
      image: "quay.io/user/argocd-agent:v1"
  sourceNamespaces:
    - "agent-managed"
    - "agent-autonomous"
```

### Step 4: Generate Agent Configurations

Create `argocd-redis` secret, because principal looks for it to fetch redis authentication details.

```bash
oc create secret generic argocd-redis -n argocd --from-literal=auth="$(oc get secret argocd-redis-initial-password -n argocd -o jsonpath='{.data.admin\.password}' | base64 -d)"
```

After this, run the agent configuration script to set up the necessary cluster secrets and other configurations.

```bash
./controllers/argocdagent/create-agent-config.sh
```
Use `--recreate` flag to recreate configurations. 

Now restart principal to pick up the new configurations. 

```bash
oc rollout -n argocd restart deployment argocd-agent-principal
```

### Step 5: Get Principal Metrics Endpoint

Retrieve the metrics endpoint URL for monitoring the principal.

```bash
echo "$(oc get svc argocd-agent-principal-metrics -n argocd -o jsonpath='{.status.loadBalancer.ingress[0].hostname}'):8000/metrics"
```

### Step 6: Set up Workload Clusters

Follow this doc (TODO: Link to helm chart documentation) to setup workload clusters.

For managed agent you need to provide host names of principal's redis and repo server in [argocd-cmd-params-cm](https://github.com/argoproj-labs/argocd-agent/blob/main/hack/dev-env/agent-managed/argocd-cmd-params-cm.yaml#L6) ConfigMap.

```bash
oc get svc argocd-redis -n argocd
oc get svc argocd-repo-server -n argocd
```

### Step 7: Issue Agent Certificates

For demo we are assuming that `Managed agent` will be deployed in `agent-managed` namespace with `vcluster-agent-managed` context and `Autonomous agent` in `argocd` namespace with `vcluster-agent-autonomous` context.

Now you need to issue agent certificates.

```bash
# For managed agent
argocd-agentctl pki issue agent agent-managed --agent-context vcluster-agent-managed --agent-namespace agent-managed --upsert

# For autonomous agent
argocd-agentctl pki issue agent agent-autonomous --agent-context vcluster-agent-autonomous --agent-namespace argocd --upsert
```

### Step 8: Start Agents in Workload Clusters

Now you can connect agents to principal.

While starting up agents you need to provide host names of principal in [agent-params-cm](https://github.com/argoproj-labs/argocd-agent/blob/main/install/kubernetes/agent/agent-params-cm.yaml#L54) ConfigMap to point it to principal server.


### Step 9: Verification

After completing the setup, verify the installation by:

1. Check in pod logs:
When an agent is connected with principal you will see log something like this:
```
level=info msg="An agent connected to the subscription stream" client=agent-managed method=Subscribe
```
2. Verify number of connected agents through `agent_connected_with_principal` metric at metrics endpoint
