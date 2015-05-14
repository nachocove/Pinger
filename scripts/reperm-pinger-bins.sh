#!/bin/sh
#
# reperm-pinger-bins.sh
# reset permissions for pinger bins and restart the services
# run this script after copying over the binaries for pinger to NACHO_GO_BIN
#
NACHO_USER=nachocove
NACHO_HOME=/home/$NACHO_USER
NACHO_GO_BIN=$NACHO_HOME/go/bin

echo ""
echo "Setting the the capacity for pinger-weserver to bind to a port less than 1024."
echo "Note: we don't need this if we are running at port 8443."
setcap 'cap_net_bind_service=+ep' $NACHO_GO_BIN/pinger-webserver 
getcap $NACHO_GO_BIN/pinger-webserver 

echo ""
echo "Restoring SELINUX security context for the files"
restorecon -v $NACHO_GO_BIN/pinger*

echo ""
echo "Restarting pinger services..."
/usr/local/bin/supervisorctl restart all

exit 0
