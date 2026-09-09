# Argo CD v3.5.2
FROM quay.io/argoproj/argocd@sha256:e2aadfae709d904e87f46ba4aa49601d827b3022db22cd4d03aae816a2e7097b AS argocd

# Final Image
FROM docker.io/library/ubuntu:26.04

USER root

ENV ARGOCD_USER_ID=999
ARG DEBIAN_FRONTEND=noninteractive

RUN groupadd -g $ARGOCD_USER_ID argocd && \
    useradd -r -u $ARGOCD_USER_ID -g argocd argocd && \
    mkdir -p /home/argocd && \
    chown argocd:0 /home/argocd && \
    chmod g=u /home/argocd && \
    apt-get update && \
    apt-get dist-upgrade -y && \
    apt-get install -y \
    git git-lfs tini curl python3-pip gpg tzdata unzip ca-certificates lsb-release && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# Create a symlink for python to point to python3
RUN ln -s /usr/bin/python3 /usr/bin/python

# Install AWS CLI (arch-aware). AWS CLI v2 only publishes x86_64 and aarch64
# binaries, so the util image targets linux/amd64 and linux/arm64.
ARG TARGETARCH
RUN AWS_ARCH=$( [ "$TARGETARCH" = "arm64" ] && echo aarch64 || echo x86_64 ); \
    curl "https://awscli.amazonaws.com/awscli-exe-linux-${AWS_ARCH}.zip" -o "awscliv2.zip" && \
    unzip awscliv2.zip && \
    ./aws/install && \
    rm -rf awscliv2.zip aws

# Install the Microsoft Azure CLI
RUN curl -sL https://aka.ms/InstallAzureCLIDeb | bash && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# Install the Google Cloud SDK (CLI)
RUN curl -sL https://sdk.cloud.google.com > /tmp/install.sh && \
    bash /tmp/install.sh --disable-prompts --install-dir=/home/argocd && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/* && \
    chown -R argocd:0 /home/argocd/google-cloud-sdk

ENV PATH="/home/argocd/google-cloud-sdk/bin:${PATH}"

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
