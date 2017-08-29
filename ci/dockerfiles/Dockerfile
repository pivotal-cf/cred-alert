FROM golang:latest
MAINTAINER PCF Security Enablement <pcf-security-enablement@pivotal.io>

ENV TERM dumb
ENV DEBIAN_FRONTEND noninteractive
ENV HOME /
ENV CLOUDSDK_PYTHON_SITEPACKAGES 1
ENV PATH /google-cloud-sdk/bin:$PATH

# Install MariaDB
RUN apt-get update && \
    apt-get -y install software-properties-common && \
    apt-key adv --recv-keys --keyserver keyserver.ubuntu.com 0xF1656F24C74CD1D8&& \
    add-apt-repository 'deb [arch=amd64] http://mirror.nodesdirect.com/mariadb/repo/10.1/debian stretch main' && \
    apt-get update && \
    apt-get -y install mariadb-server

# Install Common Dependencies
RUN echo 'deb http://httpredir.debian.org/debian testing main' >> /etc/apt/sources.list && \
    apt-get update && \
    apt-get install -y  unzip libgit2-dev pkg-config openjdk-8-jre-headless && \
    apt-get upgrade -y -qq && \
    apt-get -y clean && \
    apt-get -y autoremove --purge

# Install BOSH CLI
RUN wget https://s3.amazonaws.com/bosh-cli-artifacts/bosh-cli-2.0.28-linux-amd64 && \
    mv bosh-cli-2.0.28-linux-amd64 /usr/local/bin/bosh-cli && \
    chmod +x /usr/local/bin/bosh-cli

# Install GCloud CLI
WORKDIR /

RUN wget https://dl.google.com/dl/cloudsdk/channels/rapid/google-cloud-sdk.zip && \
    unzip google-cloud-sdk.zip && \
    rm google-cloud-sdk.zip && \
    google-cloud-sdk/install.sh --usage-reporting=false \
      --path-update=true --bash-completion=true --rc-path=/.bashrc \
      --additional-components beta pubsub-emulator && \
    google-cloud-sdk/bin/gcloud config set --installation component_manager/disable_update_check true
