FROM golang:1.7
MAINTAINER PCF Security Enablement <pcf-security-enablement@pivotal.io>

ENV TERM dumb
ENV DEBIAN_FRONTEND noninteractive

RUN apt-key adv --recv-keys --keyserver hkp://keyserver.ubuntu.com:80 0xcbcb082a1bb943db && \
    echo 'deb [arch=amd64,i386] http://nyc2.mirrors.digitalocean.com/mariadb/repo/10.1/debian jessie main' >> /etc/apt/sources.list && \
    echo 'deb-src http://nyc2.mirrors.digitalocean.com/mariadb/repo/10.1/debian jessie main' >> /etc/apt/sources.list && \
    debconf-set-selections << 'mariadb-server-10.1 mysql-server/root_password password root' && \
    debconf-set-selections << 'mariadb-server-10.1 mysql-server/root_password_again password root' && \
    apt-get update && \
    apt-get install -y --no-install-recommends mariadb-server && \
    apt-get clean && apt-get autoremove --purge && \
    rm -rf /var/lib/apt/lists/*

RUN apt-get update && \
    apt-get install -y --no-install-recommends unzip && \
    apt-get clean && apt-get autoremove --purge && \
    rm -rf /var/lib/apt/lists/*
