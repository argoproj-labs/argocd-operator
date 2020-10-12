# RHEL 8 Universal Base Image created 2020-09-01T19:43:18.550354Z
FROM registry.access.redhat.com/ubi8/ubi-minimal@sha256:5cfbaf45ca96806917830c183e9f37df2e913b187aadb32e89fd83fa455ebaa6

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
