# Argo CD v3.3.9
FROM quay.io/argoproj/argocd@sha256:fd8a938f3ec64bfa272d17c20a56b548a97711b92bd5b0a80a485e81bc724969 as argocd

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
    apt-get install -y --no-install-recommends \
      git tini curl python3-pip gpg tzdata unzip ca-certificates && \
    curl -L \
      https://github.com/git-lfs/git-lfs/releases/download/v3.7.1/git-lfs-linux-amd64-v3.7.1.tar.gz \
      -o /tmp/git-lfs.tar.gz && \
    tar -xzf /tmp/git-lfs.tar.gz -C /tmp && \
    /tmp/git-lfs-3.7.1/install.sh && \
    rm -rf /tmp/git-lfs* && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Create a symlink for python to point to python3
RUN ln -s /usr/bin/python3 /usr/bin/python

# Install AWS CLI v2
RUN curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" && \
    unzip awscliv2.zip && \
    ./aws/install && \
    rm -rf awscliv2.zip aws

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
