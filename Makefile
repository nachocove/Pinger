.PHONY:docker nachobase pinger all test install curl-register curl-defer

pinger:
	go build ./...

update:
	go get -u -f ./...

all: install test

test:
	go test -v ./...

install:
	go install ./...
	sh scripts/webserver-capabilities.sh

docker: nachobase
	docker build -t nachocove/pinger:v1 .

nachobase:
	(cd nachobase ; docker build -t nachocove/nachobase:v1 .)

curl-register:
	curl -c cookiejar -v -k -H "Content-Type: application/json" --data-binary @examples/janvD2.json https://localhost:8443/register

curl-defer:
	curl -b cookiejar -v -k -H "Content-Type: application/json" --data-binary @examples/janvD2-defer.json https://localhost:8443/defer
