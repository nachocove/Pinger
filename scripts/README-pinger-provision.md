# Pinger provisioning script
The script 'pinger-provision.py' is used to provision a Pinger deployment environment in the AWS PAAS Suite.
We use the same account 'nachocove' for both 'dev' and 'alpha' deployments. 
We have a separate account for the 'beta'/'prod' deployments.

The minimal Pinger deployment consists of an ELB and a single autoscaled instance within a VPC. 
We can run more than one instances to share load.
Currently each instance has its own SQLite DB. Soon, the instances will be sharing a DynamoDB.
Each instance keeps the user credentials provided by the clients, in memory. 
This means that the sessions from the client need to be sticky to the instance where they registered. We have turned
on stickiness at the ELB to enable that.

To create a Pinger deployment environment:
1. create a config like devprov.json or alphaprov.json (located in this directory).
2. run the create_nacho_init_.sh for the preferred deployment environment
3. run the pinger-provision.py script


## Pinger Provision script Usage
usage: pinger-provision.py [-h] [-d] --config config_file

Provision the Pinger at AWS

optional arguments:
  -h, --help            show this help message and exit
  -d, --delete          use this flag to deprovision the pinger
  --config config_file  the config(json) file for the deployment
  
## Provision a dev deployment env
1. run the following command to get the temporary security token set in your env:
   - use your own user access keys 
   - update the stscreds.sh to refer to your user id,  .aws/credentials profile_name, account_id
        USER=<user_name>
        PROFILE=<aws_credentials_profile_name>
        ACCOUNT_ID=<account_id>
   - use the MFA token as the command line argument
$ `./stscreds.sh <token_id>`

2. then run 
$ python pinger-provision.py --config devprov.json
  
  
## Provision an alpha deployment env
1. run the following command to get the temporary security token set in your env:
   - use your own user access keys 
   - update the stscreds.sh to refer to your user id,  .aws/credentials profile_name, account_id
        USER=<user_name>
        PROFILE=<aws_credentials_profile_name>
        ACCOUNT_ID=<account_id>
   - use the MFA token as the command line argument
   
2. then run 
$ python pinger-provision.py --config alphaprov.json

## To Deprovision a dev deployment env
run
$ python pinger-provision.py -d --config devprov.json

## To Deprovision an alpha deployment env
$ python pinger-provision.py -d --config alphaprov.json

## Other notes
There are a couple of timing issues with the script that need to be addressed long term.

1. Sometimes the AWS objects just created don't actually exist by the time that next command (using tagging) is run. 
   This causes the script to to bail. 
   The script is re-entrant - so just run it again.
2. When deprovisioning a deployment environment, the first action is to stop the instance and delete the autoscalar.
   This takes a while. The script bails at the subsequent steps.
   Wait for the instance to terminate (check the AWS console) and then run the script again to complete the cleanup.

  