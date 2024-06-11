FROM golang:1.20 AS builder

WORKDIR /go/src/github.com/shantanu-hashcash/go
COPY . ./
RUN go mod download
RUN go install github.com/shantanu-hashcash/go/exp/services/ledgerexporter

FROM ubuntu:22.04
ARG HCNET_CORE_VERSION
ENV HCNET_CORE_VERSION=${HCNET_CORE_VERSION:-*}
ENV HCNET_CORE_BINARY_PATH /usr/bin/hcnet-core

ENV DEBIAN_FRONTEND=noninteractive
# ca-certificates are required to make tls connections
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl wget gnupg apt-utils
RUN wget -qO - https://apt.hcnet.org/SDF.asc | APT_KEY_DONT_WARN_ON_DANGEROUS_USAGE=true apt-key add -
RUN echo "deb https://apt.hcnet.org jammy stable" >/etc/apt/sources.list.d/SDF.list
RUN echo "deb https://apt.hcnet.org jammy unstable" >/etc/apt/sources.list.d/SDF-unstable.list
RUN apt-get update && apt-get install -y hcnet-core=${HCNET_CORE_VERSION}
RUN apt-get clean

COPY --from=builder /go/src/github.com/shantanu-hashcash/go/exp/lightaurora/build/ledgerexporter/captive-core-pubnet.cfg /
COPY --from=builder /go/src/github.com/shantanu-hashcash/go/exp/lightaurora/build/ledgerexporter/captive-core-testnet.cfg /
COPY --from=builder /go/src/github.com/shantanu-hashcash/go/exp/lightaurora/build/ledgerexporter/start /

RUN ["chmod", "+x", "/start"]

# for the captive core sqlite database
RUN mkdir -p /cc

COPY --from=builder /go/bin/ledgerexporter ./

ENTRYPOINT ["/start"]
