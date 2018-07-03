FROM golang:latest
MAINTAINER PCF Security Enablement <pcf-security-enablement@pivotal.io>

ENV TERM dumb
ENV DEBIAN_FRONTEND noninteractive
ENV HOME /

# Install Common Dependencies
RUN apt-get update && \
    apt-get install -y unzip && \
    apt-get upgrade -y -qq && \
    apt-get -y clean && \
    apt-get -y autoremove --purge