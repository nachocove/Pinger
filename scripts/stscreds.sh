#!/bin/sh

if [ -z "$1" ] ; then
  echo "unset AWS_ACCESS_KEY_ID\nunset AWS_SECRET_ACCESS_KEY\nunset AWS_SESSION_TOKEN\nunset AWS_SECURITY_TOKEN\n"
  exit 0
fi

if [ "$1" == "-h" ] ; then
    echo "USAGE: $0 <aws-user>/<aws-profile>[/aws-accountId] <mfa token>"
    echo " Example for devops: $0 janv/nachocove 12345656"
    echo " To automatically set the variables use backticks: \`$0 janv/nachocove 12345\`"
    exit 0
fi

USERPROFILE=$1
TOKEN=$2
USER=`echo $USERPROFILE | cut -d'/' -f1`
PROFILE=`echo $USERPROFILE | cut -d'/' -f2`
ACCOUNT_ID=`echo $USERPROFILE | cut -d'/' -f3`
if [ -z "$ACCOUNT_ID" ] ; then
    ACCOUNT_ID=263277746520
fi


jsonResponse=`aws --profile $PROFILE sts get-session-token --serial-number arn:aws:iam::$ACCOUNT_ID:mfa/$USER --token $TOKEN`
if [ $? != 0 ] ; then
   exit 1
fi

echo $jsonResponse | python -c 'import json,sys; data=json.load(sys.stdin); print "export AWS_ACCESS_KEY_ID=%(AccessKeyId)s\nexport AWS_SECRET_ACCESS_KEY=%(SecretAccessKey)s\nexport AWS_SESSION_TOKEN=%(SessionToken)s\nexport AWS_SECURITY_TOKEN=%(SessionToken)s\n" % data["Credentials"]'
