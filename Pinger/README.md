Pinger Module
=============

RPC
---

The web-server entry-points are in rpc.go. The web-server calls the backend process via RPC, and the backend launches goroutines as appropriate.

The web-server calls functions (ia RPC) in rpc_client.go, but those are merely the actual RPC call definitions. The actual functionality starts in rpc.go.

TODO: The RPC config context is a global. Need to figure out the go-idiomatic way of passing this through to lower-level functions. From reading, go engineers at google are apparently required (or encouraged) to have a context parameter as the first argument to relevant functions, something I may have to adopt.

mailClient.go/exchange.go
-------------------------

There is a first-cut attempt at abstracting the actual mail-server details in mailClient.go. It defines an interface that any 'sub modules' must provide. exchange.go is the first (and so far only) client thereof. The interface is still a bit messy. Cleaning this up (experience tells me) takes adding a second sub-module, i.e. something for imap.

deviceInfo.go/db.go
-------------------

This is the definition of the DB table and access functions related to the DB table, as well as the lower-level DB setup functions.

aws.go
------

AWS config and access routines.

client.go
---------

My initial attempt at a go connection server. For each connection accepted, it launches 2-3 go routines. Go does not support a select on sockets. Instead, it has a select on channels. So one goroutine waits for input on the net.Conn connection (blocking) and feeds the incoming data channel. The main per-connection goroutine listens on the incoming data channel and simply echo's back what it received. It also listens on the outgoing data channel, and sends whatever it receives to the remote end. Additionally, it listens on a command channel (for things like 'Stop the go routine') and also a 'timer channel' which tells us when a timer has expired.

NOT USED by the backend currently (this could change): The exchange backend uses HTTP functions provided by go. I still need to figure out how to manage those more efficiently, keep then open (like websockets?) and reopen when necessary (currently a single connection is created, we wait for data, and then a new one is created).

"Class" Hierarchy
=================

Go doesn't have classes as such, so I use the term lightly. 

```
BackendPolling (interface)
   -> MailClientContextType (interface)
      -> MailClient (interface)
```