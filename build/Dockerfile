# RHEL 8 Universal Base Image created 2020-07-22T12:10:59.965592Z
FROM registry.access.redhat.com/ubi8/ubi-minimal@sha256:372622021a90893d9e25c298e045c804388c7666f3e756cd48f75d20172d9e55

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
