FROM golang
MAINTAINER Jan Vilhuber <janv@nachocove.com>
ENV GOPATH=/go
ADD .  /go/src/github.com/nachocove/Pinger
ADD ./config/supervisord.conf /etc/supervisord.conf
RUN go get -u github.com/nachocove/Pinger/...
RUN mkdir -p /srv/nachocove/pinger
RUN useradd nachocove
RUN chown -R nachocove /srv/nachocove
USER nachocove
ENTRYPOINT supervisord
EXPOSE 8443 8080
