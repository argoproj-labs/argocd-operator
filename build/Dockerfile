# RHEL 8 Universal Base Image created 2020-04-23T05:16:45.422878Z
FROM registry.access.redhat.com/ubi8/ubi-minimal@sha256:326c94ab44d1472a30d47c49c2f896df687184830fc66a66de00c416885125b0

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
