# Argo CD v2.4.3
FROM quay.io/argoproj/argocd@sha256:f40b85b224d52fd897e12a4f48d9bd9ba28e02f311a4a59af58a4a653e003b6f

USER root

# Ensure system dependencies are installed
RUN apt-get update && \
    apt-get install -y curl python3-pip && \
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
