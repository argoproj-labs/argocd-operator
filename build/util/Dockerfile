# Argo CD v2.1.2
FROM quay.io/argoproj/argocd@sha256:2e0e725c9282bfb7b08bea7dbbfd8dbce6410d670e3f8addd9b6540d818ad520

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
