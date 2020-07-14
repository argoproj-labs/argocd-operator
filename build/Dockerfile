# RHEL 8 Universal Base Image created 2020-06-25T19:42:34.833547Z
FROM registry.access.redhat.com/ubi8/ubi-minimal@sha256:ab6ac16b2ba297db67e036ae279f166834e8d4e881da2fcbbcfd05b44f34d415

ENV OPERATOR=/usr/local/bin/argocd-operator \
    USER_UID=1001 \
    USER_NAME=argocd-operator

# install operator binary
COPY build/_output/bin/argocd-operator ${OPERATOR}

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

# install grafana artifacts
COPY grafana /var/lib/grafana

# install redis artifacts
COPY build/redis /var/lib/redis

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
