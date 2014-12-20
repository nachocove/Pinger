Pinger
======

A place where devices register their email credentials, which
then pings the mail server at various intervals, and sends devices
push notifications.

Download:

```
go get github.com/nachocove/Pinger
```

or to get the git repo:

```
mkdir -p $GOPATH/src/github.com/nachocove/
cd $GOPATH/src/github.com/nachocove/
git clone git@github.com:nachocove/Pinger.git
```

Dependencies:

```
go get github.com/codegangsta/negroni
go get github.com/gorilla/context
go get github.com/gorilla/mux
```

To get the list of dependencies, use:
```
go list -f '{{join .Deps "\n"}}' ./... |  xargs go list -f '{{if not .Standard}}{{.ImportPath}}{{end}}'
```