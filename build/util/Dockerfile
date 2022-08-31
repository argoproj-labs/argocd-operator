# Argo CD v2.2.5
FROM quay.io/argoproj/argocd@sha256:1dbd9c885d3036f84bb4ddae0a5027b86a43801ef9af7c68092c2192451bc65c

USER root

# Ensure system dependencies are installed
RUN apt-get update && \
    apt-get install -y curl && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# Install the Google Cloud SDK (CLI)
RUN curl -sL https://sdk.cloud.google.com > /tmp/install.sh && \
    bash /tmp/install.sh --disable-prompts --install-dir=/home/argocd && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# Install the Microsoft Azure CLI
RUN curl -sL https://aka.ms/InstallAzureCLIDeb | bash && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# Copy util wrapper script
COPY util.sh /usr/local/bin/argocd-operator-util

ENV USER_NAME=argocd
ENV HOME=/home/argocd

USER argocd
WORKDIR /home/argocd
