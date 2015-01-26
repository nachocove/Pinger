Pinger Module
=============

RPC
---

The web-server entry-points are in rpc.go. The web-server calls the backend process via RPC, and the backend launches goroutines as appropriate.

The web-server calls functions (ia RPC) in rpc_client.go, but those are merely the actual RPC call definitions. The actual functionality starts in rpc.go.

mailClient.go/exchange.go
-------------------------

There is a first-cut attempt at abstracting the actual mail-server details in mailClient.go. It defines an interface that any 'sub modules' must provide. exchange.go is the first (and so far only) client thereof. The interface is still a bit messy. Cleaning this up (experience tells me) takes adding a second sub-module, i.e. something for imap.

deviceInfo.go/db.go
-------------------

This is the definition of the DB table and access functions related to the DB table, as well as the lower-level DB setup functions.