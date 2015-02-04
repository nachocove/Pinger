FROM golang
MAINTAINER Jan Vilhuber <janv@nachocove.com>
RUN apt-get update -qq -y && apt-get install -qq -y apt-utils supervisor git-core openssh-client vim less sudo
RUN addgroup nachocove
RUN adduser --ingroup nachocove --gecos "Nacho Cove User" --disabled-password nachocove
RUN adduser nachocove sudo
RUN update-rc.d supervisor defaults
RUN mkdir -p /srv/pinger
RUN mkdir /home/nachocove/.ssh
ADD deploy-key-id_rsa /home/nachocove/.ssh/id_rsa
ADD deploy-key-id_rsa.pub /home/nachocove/.ssh/id_rsa.pub
ADD deploy-key-known_hosts /home/nachocove/.ssh/known_hosts
RUN chmod 600 /home/nachocove/.ssh/id_rsa && chmod 644 /home/nachocove/.ssh/id_rsa.pub && chmod 644 /home/nachocove/.ssh/known_hosts
RUN chown -R nachocove.nachocove /srv/pinger && chown -R nachocove.nachocove /home/nachocove
