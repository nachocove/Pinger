#!/bin/sh

if [ -z $GOPATH ] ; then
    echo "ERROR: GOPATH must be set"
    exit 1
fi

sudo setcap 'cap_net_bind_service=+ep' $GOPATH/bin/pinger-webserver