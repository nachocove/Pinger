#!/bin/sh

M4=`which m4`
if [ -z "$M4" ] ; then
  echo "Could not find M4 macro utility"
  exit 1
fi

USAGE="USAGE: `basename $0` <aws_access_key_id> <aws_secret_key_id> <bucketname> <bucketprefix>
Example: ./`basename $0` AZIAJEJERJEJRJE 'jefjefjwfewj/jdjejwfjwfj' nchoconfal devpinger
"

if [ -z "$1" -o -z "$2" -o -z "$3" -o -z "$4" ] ; then
  echo "$USAGE"
  exit 1
fi

$M4 -DACCESS_KEY=$1 -DSECRET_KEY=$2 -DBUCKET=$3 -DPREFIX=$4 ../config/nacho_init.sh-template > nacho_init_$4.sh
