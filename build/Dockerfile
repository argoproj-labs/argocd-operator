# RHEL 8 Universal Base Image created 2021-05-04T17:20:18.408117Z
FROM registry.access.redhat.com/ubi8/ubi-minimal@sha256:fc75532a20c1e7eb0512a03feac46554278bce946cf454a78e11433e39a66d2d

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
