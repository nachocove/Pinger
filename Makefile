.PHONY:docker nachobase pinger all test install curl-register curl-defer clean

PINGER_HOST ?= http://localhost:8443
PINGER_AS_EXAMPLE ?= janvD2
PINGER_IMAP_EXAMPLE ?= azimGmail
PINGER_VERSION ?= 1
pinger:
	go build ./...

git-update:
	git pull
	

update-all:
	go get -u ./...

all: install test

clean:
	go clean ./...
test:
	go test ./...

vet:
	go vet ./... 2>&1 | grep -v 'possible formatting directive in Error call'
	
install: git-update
	godep restore
	go install ./...
	sh scripts/webserver-capabilities.sh

docker: nachobase
	docker build -t nachocove/pinger:v1 .

nachobase:
	(cd nachobase ; docker build -t nachocove/nachobase:v1 .)

curl-imap-register:
	curl -c /tmp/cookiejar -v -k -H "Content-Type: application/json" --data-binary @examples/$(PINGER_IMAP_EXAMPLE).json $(PINGER_HOST)/$(PINGER_VERSION)/register

curl-register:
	curl -c /tmp/cookiejar -v -k -H "Content-Type: application/json" --data-binary @examples/$(PINGER_AS_EXAMPLE).json $(PINGER_HOST)/$(PINGER_VERSION)/register

curl-defer:
	curl -b /tmp/cookiejar -v -k -H "Content-Type: application/json" --data-binary @examples/$(PINGER_AS_EXAMPLE)-defer.json $(PINGER_HOST)/$(PINGER_VERSION)/defer

curl-stop:
	curl -b /tmp/cookiejar -v -k -H "Content-Type: application/json" --data-binary @examples/$(PINGER_AS_EXAMPLE)-stop.json $(PINGER_HOST)/$(PINGER_VERSION)/stop

curl-alive:
	curl -v -k $(PINGER_HOST)/$(PINGER_VERSION)/alive?token=7701056ede5b4717babf239d215efaf0
