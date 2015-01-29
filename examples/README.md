Example payloads
================

pingerPostData.json
-------------------

an example post payload. Example usage with curl:

```
curl -v -k -H "Content-Type: application/json" --data-binary @examples/pingerPostData.json https://localhost:8443/register
```

Testing setup
-------------

Open 4 windows. In each 

```
cd $GOPATH/src/github.com/nachocove/Pinger
```

* webserver:
```
pinger-webserver -d -c config/webserver-example-config.cfg
```

* testServer (fake mail server)
```
testServer -cert config/cert.pem -key config/key.pem -d -log-level DEBUG -v -http
```

* backend
```
pinger-backend -log-level DEBUG -c config/backend-example-config.cfg -v -d
```

* "client"
```
curl -v -k -H "Content-Type: application/json" --data-binary @examples/pingerPostData.json https://localhost:8443/register
```
