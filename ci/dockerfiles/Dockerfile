FROM golang:1.7
MAINTAINER PCF Security Enablement <pcf-security-enablement@pivotal.io>

ENV TERM dumb
ENV DEBIAN_FRONTEND noninteractive
ENV HOME /
ENV CLOUDSDK_PYTHON_SITEPACKAGES 1
ENV PATH /google-cloud-sdk/bin:$PATH

RUN apt-key adv --recv-keys --keyserver hkp://keyserver.ubuntu.com:80 0xcbcb082a1bb943db && \
    echo 'deb [arch=amd64,i386] http://nyc2.mirrors.digitalocean.com/mariadb/repo/10.1/debian jessie main' >> /etc/apt/sources.list && \
    echo 'deb-src http://nyc2.mirrors.digitalocean.com/mariadb/repo/10.1/debian jessie main' >> /etc/apt/sources.list && \
    debconf-set-selections << 'mariadb-server-10.1 mysql-server/root_password password root' && \
    debconf-set-selections << 'mariadb-server-10.1 mysql-server/root_password_again password root' && \
    echo 'deb http://httpredir.debian.org/debian testing main' >> /etc/apt/sources.list && \
    apt-get update && \
    apt-get install -y --no-install-recommends mariadb-server unzip libgit2-dev pkg-config openjdk-8-jre-headless && \
    apt-get remove -y libcups2 && \
    wget http://snapshot.debian.org/archive/debian/20160809T044933Z/pool/main/g/gnutls28/libgnutls30_3.5.2-3_amd64.deb && \
    dpkg -i libgnutls30_3.5.2-3_amd64.deb && \
    go get github.com/cloudfoundry/bosh-cli && \
    rm -rf $GOPATH/src/github.com/cloudfoundry/bosh-cli libgnutls30_3.5.2-3_amd64.deb && \
    apt-get -y clean && apt-get -y autoremove --purge && \
    rm -rf /var/lib/apt/lists/*

# gcloud
WORKDIR /
RUN wget https://dl.google.com/dl/cloudsdk/channels/rapid/google-cloud-sdk.zip && \
    unzip google-cloud-sdk.zip && \
    rm google-cloud-sdk.zip && \
    google-cloud-sdk/install.sh --usage-reporting=false \
      --path-update=true --bash-completion=true --rc-path=/.bashrc \
      --additional-components beta pubsub-emulator && \
    google-cloud-sdk/bin/gcloud config set --installation component_manager/disable_update_check true
