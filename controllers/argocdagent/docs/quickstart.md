# Argo CD Agent Setup Guide

## Overview

This guide provides step-by-step instructions for setting up and configuring the Argo CD Agent in a dev environment.

## Prerequisites
- The `argocd-agentctl` CLI tool installed.
- `oc` CLI tool installed and configured. ([OpenShift CLI](https://docs.openshift.com/container-platform/latest/cli_reference/openshift_cli/getting-started-cli.html))
- `openssl` command line tool installed. ([OpenSSL Library](https://openssl-library.org/source/))

## Steps for Principal Setup

### Step 1: Create Required Namespaces

Create a namespace for the principal in the control plane cluster.
```bash
oc create ns argocd
```
Create namespaces for agents in the control plane cluster. This should be same as the [agent namespace](https://github.com/argoproj-labs/argocd-agent/blob/main/install/kubernetes/agent/agent-params-cm.yaml#L50) in workload clusters.

```bash
oc create ns agent-autonomous
oc create ns agent-managed
```

### Step 2: Start the Operator

Install the required CRDs and start the Argo CD operator in development mode.

```bash
make install
export ARGOCD_CLUSTER_CONFIG_NAMESPACES="argocd"
make run
```

### Step 3: Deploy Argo CD Instance with Principal

Use the yaml content below to create an Argo CD instance in the `argocd` namespace.

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
      server:
        auth: "mtls:CN=([^,]+)"
        logLevel: "trace"
      namespace:
        allowedNamespaces:
          - "*"
      tls:
        insecureGenerate: true   
      jwt:
        insecureGenerate: true     
  sourceNamespaces:
    - "agent-managed"
    - "agent-autonomous"
```

### Step 4: Generate Agent Configurations

Run the agent configuration script to set up the necessary cluster secrets and other configurations.

```bash
./controllers/argocdagent/scripts/create-agent-config.sh
```
Use `--recreate` flag to recreate configurations. 

Now restart principal to pick up the new configurations. 

```bash
oc rollout -n argocd restart deployment argocd-agent-principal
```

### Step 5: Create Principal Metrics Route

A Service `argocd-agent-principal-metrics` will be created to expose principal metrics. You can create a Route to expose this Service.


### Step 6: Set up Workload Clusters

Follow this [doc](https://github.com/argoproj-labs/argocd-agent/blob/main/docs/getting-started/openshift/index.md#setting-up-agent-workload-cluster) to set up workload clusters.

While installing managed agent you need to provide host names of principal's redis and repo server in [argocd-cmd-params-cm](https://github.com/argoproj-labs/argocd-agent/blob/main/hack/dev-env/agent-managed/argocd-cmd-params-cm.yaml#L6) ConfigMap to allow it to communicate with control plan's redis server.

```bash
oc get svc argocd-redis -n argocd
oc get svc argocd-repo-server -n argocd
```

### Step 7: Issue Agent Certificates

For this demo, we are assuming that the `Managed agent` will be deployed in the `agent-managed` namespace with the `vcluster-agent-managed` context and the `Autonomous agent` in the `argocd` namespace with the `vcluster-agent-autonomous` context.

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
When an agent is connected with the principal, you will see a log message something like this:
```
level=info msg="An agent connected to the subscription stream" client=agent-managed method=Subscribe
```

2. Verify the number of connected agents through the `agent_connected_with_principal` metric at the metrics endpoint
