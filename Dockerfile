FROM nachocove/nachobase:v1
MAINTAINER Jan Vilhuber <janv@nachocove.com>
ENV GOPATH=/srv/pinger/go
RUN mkdir -p /home/nachocove/config
ADD ./config /srv/pinger/config
ADD . /srv/pinger/go/src/github.com/nachocove/Pinger
RUN cp /srv/pinger/config/backend-example-config.cfg /srv/pinger/config/backend.cfg
RUN cp /srv/pinger/config/webserver-example-config.cfg /srv/pinger/config/webserver.cfg
RUN chown -R nachocove.nachocove /srv/pinger
USER nachocove
WORKDIR /srv/pinger
RUN go get -u -f github.com/nachocove/Pinger/...
RUN mkdir -p ./log
ENTRYPOINT supervisord -c ./config/supervisord.conf
#USER root
EXPOSE 8443 8080
