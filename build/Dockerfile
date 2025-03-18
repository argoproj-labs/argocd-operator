# RHEL 8 Universal Base Image created 2021-05-04T17:20:18.408117Z
FROM registry.access.redhat.com/ubi8/ubi-minimal@sha256:e4ca1025efad7cc944fb991c716067b12e25ecbe05e6e81715a337179e417aa8

ENV OPERATOR=/usr/local/bin/argocd-operator \
    USER_UID=1001 \
    USER_NAME=argocd-operator

# install operator binary
COPY build/_output/bin/argocd-operator ${OPERATOR}

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

# install redis artifacts
COPY build/redis /var/lib/redis

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
