# Argo CD v2.5.2
FROM quay.io/argoproj/argocd@sha256:9df610ec57cb8ec08001ffaeeedfe1c248e7906707a654801f8d7b4d07905693

USER root

# Ensure system dependencies are installed
RUN apt-get update && \
    apt-get install -y curl python3-pip && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# Install the AWS CLI
RUN pip3 install awscli

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
