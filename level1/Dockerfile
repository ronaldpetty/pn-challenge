FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates golang-go openssl \
  && rm -rf /var/lib/apt/lists/*

COPY scripts/init_artifacts.sh /usr/local/bin/init_artifacts.sh

RUN chmod +x /usr/local/bin/init_artifacts.sh

ENTRYPOINT ["/shared-bin/pn-demo"]
