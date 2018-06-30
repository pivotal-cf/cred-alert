FROM golang:latest
MAINTAINER PCF Security Enablement <pcf-security-enablement@pivotal.io>

ENV TERM dumb
ENV DEBIAN_FRONTEND noninteractive
ENV HOME /

# Install Common Dependencies
RUN apt-get update && \
    apt-get install -y  unzip && \
    apt-get upgrade -y -qq && \
    apt-get -y clean && \
    apt-get -y autoremove --purge

# Install Dep
RUN dep_version=$(curl --silent "https://api.github.com/repos/golang/dep/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/') && \
    wget "https://github.com/golang/dep/releases/download/${dep_version}/dep-linux-amd64" && \
    install -b dep-linux-amd64 /usr/local/bin/dep && \
    rm dep-linux-amd64