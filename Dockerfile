FROM ubuntu:22.04

RUN set -ex; \
    \
    apt-get update; \
    apt-get install -y --no-install-recommends \
        ca-certificates \
    ; \
    rm -rf /var/lib/apt/lists/*

COPY ./grafana-webhook /

ENTRYPOINT ["/grafana-webhook"]

