Pinger
======

A place where devices register their email credentials, which
then pings the mail server at various intervals, and sends devices
push notifications.

Download/install:

```
go get github.com/nachocove/Pinger/...
```

*NOTE: Yes, that's 3 dots. 3 dots ('...') is idiomatic go for 'everything under here'.*

or to get the git repo:

```
mkdir -p $GOPATH/src/github.com/nachocove/
cd $GOPATH/src/github.com/nachocove/
git clone git@github.com:nachocove/Pinger.git
```

Links about performance and scaling:

"Alternative to a go routine per connection?" https://groups.google.com/forum/#!topic/golang-nuts/TSf14CJyA2s

Recommends just sticking with one goroutine per connection. Seems it can handle 100K (but didn't say how much memory that uses).

Web server Dependencies (will get pulled in with 'go get ...' during the initial pull of Pinger):

```
code.google.com/p/gcfg
code.google.com/p/gcfg/scanner
code.google.com/p/gcfg/token
code.google.com/p/gcfg/types
github.com/codegangsta/negroni
github.com/gorilla/context
github.com/gorilla/mux
github.com/op/go-logging
```

To get the list of dependencies, use:
```
go list -f '{{join .Deps "\n"}}' ./... |  xargs go list -f '{{if not .Standard}}{{.ImportPath}}{{end}}'
```
