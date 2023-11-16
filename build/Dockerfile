# RHEL 8 Universal Base Image created 2021-05-04T17:20:18.408117Z
FROM registry.access.redhat.com/ubi8/ubi-minimal@sha256:87bcbfedfd70e67aab3875fff103bade460aeff510033ebb36b7efa009ab6639

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
