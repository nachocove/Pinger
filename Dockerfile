FROM golang
MAINTAINER Jan Vilhuber <janv@nachocove.com>
ADD .  /go/src/github.com/nachocove/Pinger
RUN go get github.com/nachocove/Pinger/...
RUN mkdir -p /srv/nachocove/testServer
RUN useradd nachocove
RUN chown -R nachocove /srv/nachocove
USER nachocove
ENTRYPOINT /go/bin/testServer -v
EXPOSE 8082
