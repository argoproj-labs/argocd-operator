# RHEL 8 Universal Base Image created 2020-06-22T07:39:38.453712Z
FROM registry.access.redhat.com/ubi8/ubi-minimal@sha256:45efa640a7a50ec490d22f4e48c8a6e4892ddfee7ebe58ae9453c870e3ad3e00

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
