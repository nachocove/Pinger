Pinger
======

A place where devices register their email credentials, which
then pings the mail server at various intervals, and sends devices
push notifications.

Download/install
----------------

```
go get [-u] github.com/nachocove/Pinger/...
```
Add the -u to update an existing install

*NOTE: Yes, that's 3 dots. 3 dots ('...') is idiomatic go for 'everything under here'.*


Git repo
--------

```
mkdir -p $GOPATH/src/github.com/nachocove/
cd $GOPATH/src/github.com/nachocove/
git clone git@github.com:nachocove/Pinger.git
```

Architecture
------------

The web-server is designed to not have any DB connectivity and cause no DB calls. Nor should the web server (if possible) do any calls to Amazon or anything else. Instead, the web-server can make one of very few RPC calls to the backend. The backend can then 'spawn' goroutines to do heavy lifting. Currently, the RPC calls are synchronous and return once the goroutines are launched. Most of the work that would cause connections (like DB or amazon or push, or whatever) happen in the goroutine and thus don't impact the web-server waiting. This could be changed as necesary, but currently the idea is that the webserver runs fast and small.

Reasons for a small, light web-server:

* less code-footprint. The less it does, the less of a security atteack-surface we present. Anyone managing to somehow read memory of the web-server (see HeartBleed-like attacks) can NOT get any AWS credentials, no user mail-credentials, etc.

* fast means less issue with DoS attacks. DoS attacks can not be guarded against (it's a numbers game), but the less work we do on the web-server, the less we can be DoS'd. A DoS attak on the server could potentially affect the backend (more RPC calls and go routines), but if the backend is fast and does quick checks up-front, then we can more quickly terminate the goroutines. Goroutines are much more light-weight than threads, so any additional thread does NOT take up a significant amount of memory.

Links
-----

Links about performance and scaling:

"Alternative to a go routine per connection?" https://groups.google.com/forum/#!topic/golang-nuts/TSf14CJyA2s

Recommends just sticking with one goroutine per connection. Seems it can handle 100K (but didn't say how much memory that uses).

Dependencies
------------

Dependencies are generated with the 'godep' program (https://github.com/tools/godep).

After adding another package, run
```
godep save
```

and add and commit everything under the ./GoDeps directory.

To restore (on a builder, for example):

```
godep restore
```
 