
FROM argoproj/argocd:v1.5.2

USER root

# Ensure apt-transport-https is installed
RUN apt-get update && \
    apt-get install -y \
    apt-transport-https \
    ca-certificates \
    curl \
    gnupg

# Import the Google Cloud public key
RUN curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key --keyring /usr/share/keyrings/cloud.google.gpg add -

# Add the Cloud SDK distribution URI as a package source
COPY util/google-cloud-sdk.list /etc/apt/sources.list.d/google-cloud-sdk.list

# Update and install the Google Cloud SDK
RUN apt-get update && \
    apt-get install -y google-cloud-sdk && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# Copy util wrapper script
COPY util/util.sh /usr/local/bin/argocd-operator-util

USER argocd
