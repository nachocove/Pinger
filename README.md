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

Pinger is split into two programs: `pinger-webserver` and `pinger-backend`.

pinger-webserver
++++++++++++++++

The web-server is designed to not have any DB connectivity and cause no DB calls. Nor should the web server (if possible) do any calls to Amazon or anything else. Instead, the web-server can make one of very few RPC calls to the backend.

Reasons for a small, light web-server:

* less code-footprint. The less it does, the less of a security atteack-surface we present. Anyone managing to somehow read memory of the web-server (see HeartBleed-like attacks) can NOT get any AWS credentials, no user mail-credentials, etc.

* fast means less issue with DoS attacks. DoS attacks can not be guarded against (it's a numbers game), but the less work we do on the web-server, the less we can be DoS'd. A DoS attak on the server could potentially affect the backend (more RPC calls and go routines), but if the backend is fast and does quick checks up-front, then we can more quickly terminate the goroutines. Goroutines are much more light-weight than threads, so any additional thread does NOT take up a significant amount of memory.

pinger-backend
++++++++++++++

The backend will 'spawn' goroutines to do heavy lifting from the RPC calls. Currently, the RPC calls are synchronous and return once the goroutines are launched. Most of the work that would cause connections (like DB or amazon or push, or whatever) happen in the goroutine and thus don't impact the web-server waiting. This could be changed as necessary, but currently the idea is that the webserver runs fast and small.


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

Operations
----------

The pinger programs (pinger-webserver and pinger-backend) are managed by supervisord (http://supervisord.org/). The config file for supervisor is config/supervisord.conf.

Supervisor monitors the programs it backgrounds and restarts as necessary. It can (but currently doesn't) send email when a restart happens. It can (but currently doesn't)
send email whenever there is anything on stdout and stderr from either program. So far, the webserver seems to get a lot of SSL/TLS warnings from (rogue?) access to the server.

To restart on a pinger:
```
supervisorctl restart all
```

Deployment of new code
++++++++++++++++++++++

This is currently still a manual process. Eventually, we want to use docker or some other packaging system, but currently that's
more trouble than its worth. To deploy new code:

```
cd $PINGER_HOME
make install
```

you will be asked for your github credentials (which are not saved!) for the git pull (in the Makefile). Next, you will be asked for
the nachocove user's password for a sudo command that gives the webserver permissions to listen on port 443 and 80.
After the `make install` succeeds, do a `supervisorctl restart all`.