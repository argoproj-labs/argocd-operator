# RHEL 8 Universal Base Image created 2021-05-04T17:20:18.408117Z
FROM registry.access.redhat.com/ubi8/ubi-minimal@sha256:b93deceb59a58588d5b16429fc47f98920f84740a1f2ed6454e33275f0701b59

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
