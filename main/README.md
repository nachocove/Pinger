Various programs
================

A 'main' program in go is any package that defines package 'main' and has a 'main' function. These are all nested under this folder

pinger-backend
--------------

The main backend process. This is what does all the heavy lifting. See config/backend-example-config.cfg for an example config that the backend will need.

pinger-webserver
----------------

The internet facing web-server that provides the API's that clients iwll call. It calls the backend via RPC. See config/webserver-example-config.cfg for an example config that the webserver will need. config/ also contains some self-signed certs that can be used for SSL/TLS.

testClient
----------

A test client used for POC'ing go in general. It loops and launches connections, and does various timing tasks.

testServer
----------

A test server used by the testClient to measure Go speed and efficiency. Can also be used as an echo http server to test the backend, for example.
