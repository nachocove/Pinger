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

Links about performance and scaling:

"Alternative to a go routine per connection?" https://groups.google.com/forum/#!topic/golang-nuts/TSf14CJyA2s

Recommends just sticking with one goroutine per connection. Seems it can handle 100K (but didn't say how much memory that uses).

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