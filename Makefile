.PHONY:docker nachobase pinger all test install curl-register curl-defer

PINGER_HOST ?= localhost:8443
PINGER_VERSION ?= 1
pinger:
	go build ./...

update:
	go get -u ./...

all: install test

test:
	go test -v ./...

vet:
	go vet ./... 2>&1 | grep -v 'possible formatting directive in Error call'
install:
	go install ./...
	sh scripts/webserver-capabilities.sh

docker: nachobase
	docker build -t nachocove/pinger:v1 .

nachobase:
	(cd nachobase ; docker build -t nachocove/nachobase:v1 .)

curl-register:
	curl -c /tmp/cookiejar -v -k -H "Content-Type: application/json" --data-binary @examples/janvD2.json https://$(PINGER_HOST)/$(PINGER_VERSION)/register

curl-defer:
	curl -b /tmp/cookiejar -v -k -H "Content-Type: application/json" --data-binary @examples/janvD2-defer.json https://$(PINGER_HOST)/$(PINGER_VERSION)/defer

curl-stop:
	curl -b /tmp/cookiejar -v -k -H "Content-Type: application/json" --data-binary @examples/janvD2-stop.json https://$(PINGER_HOST)/$(PINGER_VERSION)/stop
