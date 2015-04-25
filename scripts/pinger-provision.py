__author__ = 'azimo'
import sys
import time
import traceback
import boto
import boto.vpc
import argparse
import json
from pprint import pprint
from boto.s3.key import Key
import boto.ec2
import boto.ec2.elb
from boto.ec2.elb import HealthCheck
import boto.ec2.autoscale
from boto.exception import S3ResponseError, EC2ResponseError, BotoServerError
from boto.ec2.autoscale import LaunchConfiguration
from boto.ec2.autoscale import AutoScalingGroup
from boto.vpc import VPCConnection

# get region from region_name
def get_region(region_name):
    for region in boto.ec2.regions():
        if region_name == region.name:
            return region

# wait for the VPC to get out of 'pending' state
def wait_for_vpc (c, vpc_id):
    # sometimes the vpc takes a bit to get created. try thrice
    vpc_list = []
    for x in range(0, 3):
        try:
            vpc_list = c.get_all_vpcs(vpc_ids=[vpc_id])
            break
        except EC2ResponseError:
            print "Waiting for VPC(%s) to be created" % vpc_id
            time.sleep(1)
    if not len(vpc_list):
        raise Exception("Error:Cannot find the VPC(%s) just created" % vpc_id)
    while vpc_list[0].state == 'pending':
        print "Waiting for VPC(%s) to be get out of pending state" % vpc_id
        time.sleep(1)
        vpc_list = c.get_all_vpcs(vpc_ids =[vpc_id])

# get vpc by name
def get_vpc_by_name(conn, name):
    vpc_list = conn.get_all_vpcs()
    for vpc in vpc_list:
        if 'Name' in vpc.tags:
            if vpc.tags['Name'] == name:
                return vpc
    return None

# delete VPC
def delete_vpc(region, name):
    conn = VPCConnection(region=region, profile_name="provisioner")
    vpc = get_vpc_by_name(conn, name)
    if not vpc:
        print "VPC %s does not exist. Nothing to delete" % name
    else:
        print "Deleting VPC %s..." % name
        conn.delete_vpc(vpc.id)

# create VPC
def create_vpc(conn, name, cidr_block, instance_tenancy):
    print "Creating vpc %s" % name
    vpc = get_vpc_by_name(conn, name)
    if not vpc:
        vpc = conn.create_vpc(cidr_block, instance_tenancy=instance_tenancy)
        print "Created VPC %s (%s) for cidr_block: %s" % (name, vpc.id, vpc.cidr_block)
        wait_for_vpc(conn, vpc.id)
        vpc.add_tag("Name", name)
    else:
        print "VPC %s (%s) already exists at cidr_block: %s!" % (name, vpc.id, vpc.cidr_block)
    return vpc

# create internet gateway
def create_ig(conn, vpc, name):
    print "Creating internet gateway %s" % name
    ig_list = conn.get_all_internet_gateways(filters=[("attachment.vpc-id", vpc.id)])
    if not len(ig_list):
        ig = conn.create_internet_gateway()
        conn.attach_internet_gateway(ig.id, vpc.id)
        ig.add_tag("Name", name)
        print "Created Internet Gateway %s (%s) for VPC(%s)!" % (name, ig.id, vpc.id)
    else:
        ig = ig_list[0]
        print "Internet Gateway %s (%s) already exists for VPC(%s)!" % (name, ig.id, vpc.id)

    return ig

# wait for subnet to be created
def wait_for_subnet(c, sn_id):
    subnet_list = c.get_all_subnets(subnet_ids=[sn_id])
    while subnet_list[0].state == 'pending':
        print "waiting for subnet(%s) to be created" % sn_id
        time.sleep(1)
        subnet_list = c.get_all_subnets(subnet_ids =[sn_id])

# create subnet
def create_subnet(conn, vpc, name, cidr_block, availability_zone):
    print "Creating subnet %s" % name
    subnet_list = conn.get_all_subnets(filters=[("cidrBlock", cidr_block), ("vpcId", vpc.id),
                                                ("availabilityZone", [availability_zone])])
    if not len(subnet_list):
        subnet = conn.create_subnet(vpc.id, cidr_block, availability_zone=availability_zone)
        wait_for_subnet(conn, subnet.id)
        subnet.add_tag("Name", name)
        print "Created subnet %s (%s) for cidr_block: %s with available IPs: %d" % (name, subnet.id, subnet.cidr_block, subnet.available_ip_address_count)
    else:
        subnet = subnet_list[0]
        print "Subnet %s (%s) already exists at cidr_block: %s!" % (name, subnet.id, subnet.cidr_block)
    return subnet

# get security group by name within the VPC
def get_sg_by_name(conn, vpc, name):
    sg_list = conn.get_all_security_groups(filters=[("vpc-id", vpc.id)])
    for sg in sg_list:
        if sg.name == name:
            return sg
    return None

# update routing table for VPC
def update_route_table(conn, vpc, ig, name):
    print "Updating route table %s" % name
    rt_list = conn.get_all_route_tables(filters=[("vpc-id", vpc.id)])
    if not len(rt_list):
        print "Cannot find default route table for VPC(%s)" % vpc.id
        rt = None
    else:
        rt = rt_list[0]
        print "Route Table %s (%s) found for VPC(%s)" % (name, rt.id, vpc.id)
        rt.add_tag("Name", name)
        status = conn.create_route(rt.id, "0.0.0.0/0", ig.id)
        print "Added Route (%s) to Route Table %s (%s). Status %s" % ("0.0.0.0/0", name, rt.id, status)
    return rt

# create Security Group
# TODO : figure out how to create outbound rules
def create_sg(conn, vpc, name, description):
    print "Creating security group %s" % name
    sg = get_sg_by_name(conn, vpc, name)
    if not sg:
        sg = conn.create_security_group(name, description, vpc.id)
        print "Created Security Group %s (%s) for VPC(%s)" % (name, sg.id, vpc.id)
        sg.add_tag("Name", name)
    else:
        print "Security Group %s (%s) found for VPC(%s)" % (sg.name, sg.id, vpc.id)
    return sg

# check if security group rule exists
def sg_rule_exists(sg, rule):
    for r in sg.rules:
        if r.ip_protocol == rule["protocol"] and  int(r.from_port) == rule["from_port"] and\
                int(r.to_port) == rule["to_port"] and r.grants[0].cidr_ip == rule["cidr_ip"]:
            return True
    return False

# add rules to sg
def add_rules_to_sg(conn, sg, rules):
    print "Adding rules to security group"
    for rule in rules:
        if (sg_rule_exists(sg, rule)):
            print "Rule [(%s)-from_port-(%s)-to_port-(%s)-allow-access(%s) exists." % (rule["protocol"], rule["from_port"], rule["to_port"], rule["cidr_ip"])
        else:
            sg.authorize(ip_protocol=rule["protocol"], from_port=rule["from_port"], to_port=rule["to_port"], cidr_ip=rule["cidr_ip"])
            print "Rule [(%s)-from_port-(%s)-to_port-(%s)-allow-access(%s) added." % (rule["protocol"], rule["from_port"], rule["to_port"], rule["cidr_ip"])

# delete auto scale and launch configuration
def delete_autoscaler(region_name, name):
    conn = boto.ec2.autoscale.connect_to_region(region_name, profile_name="provisioner")
    asg_list = conn.get_all_groups(names=[name])
    if not len(asg_list):
        print "Auto Scaler %s does not exist. Nothing to delete" % name
    else:
        print "Deleting auto scaler %s..." % name
        conn.delete_auto_scaling_group(name, force_delete=True)
    lc_name = name + "-LC"
    lc_list = conn.get_all_launch_configurations(names=[lc_name])
    if not len(lc_list):
        print "Launch Configuration %s does not exist. Nothing to delete" % lc_name
    else:
        print "Deleting launch configuration %s..." % lc_name
        conn. delete_launch_configuration(lc_name)

# create auto scaler
def create_autoscaler(region_name, vpc, elb, subnet, sg, name, aws_config, as_config):
    print "Creating auto scaler %s" % name
    conn = boto.ec2.autoscale.connect_to_region(region_name, profile_name="provisioner")
    asg_list = conn.get_all_groups(names=[name])
    if not len(asg_list):
        with open (as_config["user_data_file"], "r") as udfile:
            user_data = udfile.read()
        lc_name = name + "-LC"
        lc_list = conn.get_all_launch_configurations(names=[lc_name])
        if not len(lc_list):
            print "Creating Launch Configuration (%s)" % lc_name
            lc = LaunchConfiguration(name=lc_name, image_id=as_config["ami_id"],
                key_name=as_config["key_pair"],
                security_groups=[sg.id],
                user_data = user_data,
                instance_type = as_config["instance_type"],
                instance_monitoring = as_config["instance_monitoring"],
                associate_public_ip_address = True
                )
            conn.create_launch_configuration(lc)
        else:
            lc=lc_list[0]
            print "Launch Configuration (%s) already exists" % lc_name
        tag = boto.ec2.autoscale.tag.Tag(key="Name", value=name +"Instance",
             propagate_at_launch=True, resource_id=name)
        asg = AutoScalingGroup(group_name=name, load_balancers=[elb.name],
            availability_zones=aws_config["zones"],
            launch_config=lc, min_size=as_config["min_size"], max_size=as_config["max_size"],
            vpc_zone_identifier = [subnet.id],
            tags=[tag],
            connection=conn)
        conn.create_auto_scaling_group(asg)
        print "Created Auto Scaler Group (%s) for VPC(%s)" % (asg.name, vpc.id)
    else:
        asg = asg_list[0]
        print "Auto Scaler Group (%s) found for VPC(%s)" % (asg.name, elb.vpc_id)
    for act in conn.get_all_activities(asg):
        print "Activiity %s" % act
    return asg

# delete load balancer
def delete_elb(region_name, name):
    conn = boto.ec2.elb.connect_to_region(region_name, profile_name="provisioner")
    try:
        elb_list = conn.get_all_load_balancers(load_balancer_names=[name])
    except BotoServerError, e: # ELB by the given name does not exist
        elb_list = []
    if not len(elb_list):
        print "Elastic Load Balancer %s does not exist. Nothing to delete" % name
    else:
        print "Deleting Elastic Load Balancer %s..." % name
        conn.delete_load_balancer(name)

# TODO 1 - configure the ELB to check the certificate on the instance - SetLoadBalancerPoliciesForBackendServer
# http://docs.aws.amazon.com/ElasticLoadBalancing/latest/APIReference/API_SetLoadBalancerPoliciesForBackendServer.html
# TODO 2 - figure out a workaround for the HTTP bug in boto's listener config
# create load balancer
def create_elb(region_name, vpc, subnet, sg, name, config):
    print "Creating elastic load balancer"
    conn = boto.ec2.elb.connect_to_region(region_name, profile_name="provisioner")
    try:
        elb_list = conn.get_all_load_balancers(load_balancer_names=[name])
    except BotoServerError, e: # ELB by the given name does not exist
        elb_list = []
    if not len(elb_list):
        ports = config["ports"]
        elb = conn.create_load_balancer(name, None, listeners=ports, subnets=[subnet.id], security_groups=[sg.id])
        hc = HealthCheck(
            interval = config["health_check"]["interval"],
            healthy_threshold = config["health_check"]["healthy_threshold"],
            unhealthy_threshold = config["health_check"]["unhealthy_threshold"],
            target = config["health_check"]["target"]
        )
        elb.configure_health_check(hc)
        print "Created Elastic Load Balancer (%s) for VPC(%s)" % (elb.name, vpc.id)
    else:
        elb = elb_list[0]
        print "Elastic Load Balancer (%s) found for VPC(%s)" % (elb.name, elb.vpc_id)
        if (elb.vpc_id != vpc.id):
            raise Exception("Error: Wrong VPC association: ELB(%s) is associated with VPC(%s) rather than VPC(%s)" % (elb.name, elb.vpc_id, vpc.id))
    #elb.register_instances(ins.id)
    return elb

# cleanup
def cleanup(config):
    print "Cleaning up..."
    deprovision_pinger(config)

# process config
def process_config(config):
    config["aws_config"]["region"] = get_region(config["aws_config"]["region_name"])

# load config from S3
def load_config_from_s3(s3_config):
    conn = boto.connect_s3(profile_name="provisioner")
    bucket = boto.s3.bucket.Bucket(conn, s3_config["s3_bucket"])
    s3_files = dict ()
    for key in s3_config["s3_filenames"]:
        s3_key = Key(bucket, s3_config["s3_filenames"][key])
        s3_files[key] = s3_key.get_contents_as_string()
    s3_config["s3_files"] = s3_files

# load json config
def json_config(file_name):
    with open(file_name) as data_file:
        json_data = json.load(data_file)
    #pprint json_data
    return json_data

# delete VPC et al
def deprovision_pinger(config):
    aws_config = config["aws_config"]
    s3_config = config["s3_config"]
    vpc_config = config["vpc_config"]
    as_config = config["autoscale_config"]
    elb_config = config["elb_config"]
    print "De-Provisioning Pinger %s" % vpc_config["name"]
    delete_autoscaler(aws_config["region_name"], vpc_config["name"] + "-AS")
    delete_elb(aws_config["region_name"], vpc_config["name"] + "-ELB")
    delete_vpc(aws_config["region"], vpc_config["name"])

# create VPC et al
def provision_pinger(config):
    aws_config = config["aws_config"]
    s3_config = config["s3_config"]
    vpc_config = config["vpc_config"]
    as_config = config["autoscale_config"]
    elb_config = config["elb_config"]
    #load_config_from_s3(s3_config)

    print "Provisioning Pinger %s" % vpc_config["name"]
    # create connection
    conn = VPCConnection(region=aws_config["region"], profile_name="provisioner")
    # create vpc
    try:
        vpc = create_vpc(conn, vpc_config["name"], vpc_config["vpc_cidr_block"], vpc_config["instance_tenancy"])
        subnet = create_subnet(conn, vpc, vpc_config["name"]+"-SN", vpc_config["subnet_cidr_block"], vpc_config["availability_zone"])
        ig = create_ig(conn, vpc, vpc_config["name"]+"-IG")
        rt = update_route_table(conn, vpc, ig, vpc_config["name"]+"-RT")
        elb_sg_config = config["elb_config"]["sg_config"]
        elb_sg = create_sg(conn, vpc, vpc_config["name"]+elb_sg_config["name"]+"-SG", elb_sg_config["description"] + " for " + vpc_config["name"])
        add_rules_to_sg(conn, elb_sg, elb_sg_config["rules"])
        elb = create_elb(aws_config["region_name"], vpc, subnet, elb_sg, vpc_config["name"] + "-ELB", elb_config)
        ins_sg_config = config["autoscale_config"]["sg_config"]
        ins_sg = create_sg(conn, vpc, vpc_config["name"]+ins_sg_config["name"]+"-SG", ins_sg_config["description"] + " for " + vpc_config["name"])
        add_rules_to_sg(conn, ins_sg, ins_sg_config["rules"])
        ascaler = create_autoscaler(aws_config["region_name"], vpc, elb, subnet, ins_sg, vpc_config["name"] + "-AS", aws_config, as_config)
    except (BotoServerError, S3ResponseError, EC2ResponseError) as e:
        print "Error :%s(%s):%s" % (e.error_code, e.status, e.message)
        print traceback.format_exc()
        cleanup(config)

# main
def main():
    parser = argparse.ArgumentParser(description='Provision the Pinger at AWS')
    parser.add_argument('-d', '--delete', help='use this flag to deprovision the pinger', action='store_true')
    parser.add_argument('--config', required=True, type=json_config, metavar = "config_file",
                   help='the config(json) file for the deployment', )
    args = parser.parse_args()
    config = args.config
    process_config(config)
    if args.delete:
        deprovision_pinger(config)
    else:
        provision_pinger(config)

if __name__ == '__main__':
    main()