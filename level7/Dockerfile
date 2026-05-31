FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates golang-go \
  && rm -rf /var/lib/apt/lists/*

ENTRYPOINT ["/shared-bin/level7-demo"]
