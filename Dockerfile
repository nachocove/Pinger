FROM golang
ADD .  /go/src/github.com/nachocove/Pinger
RUN go get github.com/nachocove/Pinger/...
ENTRYPOINT /go/bin/testServer
EXPOSE 8082
