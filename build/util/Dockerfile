# Argo CD v2.11.2
FROM quay.io/argoproj/argocd@sha256:5c9ebcabee248dc2963080365e3ab6a360a44e668a6d90f578be1a4e78623df3 as argocd

# Final Image
FROM docker.io/library/ubuntu:24.04

USER root

ENV ARGOCD_USER_ID=999
ENV DEBIAN_FRONTEND noninteractive

RUN groupadd -g $ARGOCD_USER_ID argocd && \
    useradd -r -u $ARGOCD_USER_ID -g argocd argocd && \
    mkdir -p /home/argocd && \
    chown argocd:0 /home/argocd && \
    chmod g=u /home/argocd && \
    apt-get update && \
    apt-get dist-upgrade -y && \
    apt-get install -y \
    git git-lfs tini curl python3-pip gpg tzdata && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# Install the AWS CLI
RUN pip3 install awscli

# Install the Microsoft Azure CLI
RUN curl -sL https://aka.ms/InstallAzureCLIDeb | bash && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# Install the Google Cloud SDK (CLI)
RUN curl -sL https://sdk.cloud.google.com > /tmp/install.sh && \
    bash /tmp/install.sh --disable-prompts --install-dir=/home/argocd && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

ENV PATH="/usr/local/google-cloud-sdk/bin:${PATH}" 

COPY entrypoint.sh /usr/local/bin/uid_entrypoint.sh
RUN chmod +x /usr/local/bin/uid_entrypoint.sh

# Install Argo CD CLI from argocd
COPY --from=argocd /usr/local/bin/argocd /usr/local/bin/argocd

# Copy util wrapper script
COPY util.sh /usr/local/bin/argocd-operator-util

ENV USER_NAME=argocd
ENV HOME=/home/argocd

USER argocd
WORKDIR /home/argocd
