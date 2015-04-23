__author__ = 'azimo'
import sys
import time
import boto
import boto.vpc
import argparse
import json
from pprint import pprint
from boto.s3.key import Key
import boto.ec2

def get_region(region_name):
    for region in boto.ec2.regions():
        if region_name == region.name:
            return region

def wait_for_vpc (c, id):
    vpc_list = c.get_all_vpcs(vpc_ids=[id])
    while vpc_list[0].state == 'pending':
        print "waiting for VPC(%s) to be created" % id
        time.sleep(1)
        vpc_list = c.get_all_vpcs(vpc_ids =[id])

def create_vpc(conn, name, cidr_block, instance_tenancy):
    vpc_list = conn.get_all_vpcs(filters=[("cidrBlock", cidr_block)])
    if not len(vpc_list):
        vpc = conn.create_vpc(cidr_block, instance_tenancy = instance_tenancy)
        print "Created VPC (%s) for cidr_block: %s" % (vpc.id, vpc.cidr_block)
        wait_for_vpc(conn, vpc.id)
        vpc.add_tag("Name", name)
    else:
        vpc = vpc_list[0]
        print "VPC (%s) already exists at cidr_block: %s!" % (vpc.id, vpc.cidr_block)
    return vpc

def create_ig(conn, vpc, name):
    ig_list = conn.get_all_internet_gateways(filters=[("attachment.vpc-id", vpc.id)])
    if not len(ig_list):
        ig = conn.create_internet_gateway()
        conn.attach_internet_gateway(ig.id, vpc.id)
        ig.add_tag("Name", name)
        print "Created IG (%s) for VPC(%s)!" % (ig.id, vpc.id)
    else:
        ig = ig_list[0]
        print "IG (%s) already exists for VPC(%s)!" % (ig.id, vpc.id)

    return ig

def wait_for_subnet (c, id):
    subnet_list = c.get_all_subnets(subnet_ids=[id])
    while subnet_list[0].state == 'pending':
        print "waiting for subnet(%s) to be created" % id
        time.sleep(1)
        subnet_list = c.get_all_subnets(subnet_ids =[id])

def create_subnet(conn, vpc, name, cidr_block):
    subnet_list = conn.get_all_subnets(filters=[("cidrBlock", cidr_block), ("vpcId", vpc.id)])
    if not len(subnet_list):
        subnet = conn.create_subnet(vpc.id, cidr_block)
        wait_for_subnet(conn, subnet.id)
        subnet.add_tag("Name", name)
        print "Created subnet (%s) for cidr_block: %s with available IPs: %d" % (subnet.id, subnet.cidr_block, subnet.available_ip_address_count)
    else:
        subnet = subnet_list[0]
        print "Subnet (%s) already exists at cidr_block: %s!" % (subnet.id, subnet.cidr_block)
    return subnet


def create_sg(conn, vpc, name, description):
    sg_list = conn.get_all_security_groups(filters=[("vpc-id", vpc.id)])
    if not len(sg_list):
        sg = conn.create_security_group(name, description, vpc.id)
        sg.add_tag("Name", name)
        print "Created Security Group (%s) for VPC(%s)" % (sg.id, vpc.id)
    else:
        sg = sg_list[0]
        print "Security Group (%s) found for VPC(%s)" % (sg.id, vpc.id)
        sg.add_tag("Name", name)
    return sg

def update_route_table(conn, vpc, ig, name):
    rt_list = conn.get_all_route_tables(filters=[("vpc-id", vpc.id)])
    if not len(rt_list):
        print "Cannot find default route table for VPC(%s)" % vpc.id
        rt = None
    else:
        rt = rt_list[0]
        print "Route Table (%s) found for VPC(%s)" % (rt.id, vpc.id)
        rt.add_tag("Name", name)
        status = conn.create_route(rt.id, "0.0.0.0/0", ig.id)
        print "Added Route (%s) to Route Table(%s). Status %s" % ("0.0.0.0/0", rt.id, status)
    return rt

def sg_rule_exists(sg, rule):
    for r in sg.rules:
        if r.ip_protocol == rule["protocol"] and  int(r.from_port) == rule["from_port"] and\
                int(r.to_port) == rule["to_port"] and r.grants[0].cidr_ip == rule["cidr_ip"]:
            return True
    return False


def add_rules_to_sg(conn, sg, rules):
    for rule in rules:
        if (sg_rule_exists(sg, rule)):
            print "Rule [(%s)-from_port-(%s)-to_port-(%s)-allow-access(%s) exists." % (rule["protocol"], rule["from_port"], rule["to_port"], rule["cidr_ip"])
        else:
            sg.authorize(ip_protocol=rule["protocol"], from_port=rule["from_port"], to_port=rule["to_port"], cidr_ip=rule["cidr_ip"])
            print "Rule [(%s)-from_port-(%s)-to_port-(%s)-allow-access(%s) added." % (rule["protocol"], rule["from_port"], rule["to_port"], rule["cidr_ip"])

def create_instance(conn, vpc, sg, subnet, name, config):
    ins_list = conn.get_all_reservations(filters=[("vpc-id", vpc.id)])
    if not len(ins_list):
        reservation = conn.run_instances(config["ami_id"], key_name=config["key_pair"],
            security_group_ids=[sg.id],
            instance_type=config["type"],
            subnet_id=subnet.id)
        ins = reservation.instances[0]
        print "Created Instance(%s). Status:(%s)" % (ins.id, ins.state)
    else:
        ins = ins_list[0].instances[0]
        print "Instance(%s) already exists. Status:(%s)" % (ins.id, ins.state)

    # Wait for the instance to be running
    while ins.state == 'pending':
        print "Waiting for instance(%s) to get out of pending. Status:(%s)" % (ins.id, ins.state)
        time.sleep(1)
        ins.update()
    ins.add_tag("Name", name)
    eip = conn.allocate_address(domain='vpc')
    conn.associate_address(instance_id=ins.id, allocation_id=eip.allocation_id)
    print "Instance(%s) status is :(%s)" % (ins.id, ins.state)

def process_config(config):
    config["aws_config"]["region"] = get_region(config["aws_config"]["region_name"])

def load_config_from_s3(s3_config):
    conn = boto.connect_s3()
    bucket = boto.s3.bucket.Bucket(conn, s3_config["s3_bucket"])
    s3_files = dict ()
    for key in s3_config["s3_filenames"]:
        s3_key = Key(bucket, s3_config["s3_filenames"][key])
        s3_files[key] = s3_key.get_contents_as_string()
    s3_config["s3_files"] = s3_files

def json_config(file_name):
    with open(file_name) as data_file:
        json_data = json.load(data_file)
    #pprint json_data
    return json_data

def main():
    parser = argparse.ArgumentParser(description='Provision the Pinger at AWS')
    parser.add_argument('--config', required=True, type=json_config, metavar = "config_file",
                   help='the config(json) file for the deployment', )
    args = parser.parse_args()
    config =  args.config

    process_config(config)
    aws_config = config["aws_config"]
    s3_config = config["s3_config"]
    vpc_config = config["vpc_config"]
    sg_config = config["sg_config"]
    ins_config = config["ins_config"];

    load_config_from_s3(s3_config)
    pprint(config)

    # create connection
    from boto.vpc import VPCConnection
    conn = VPCConnection(region=aws_config["region"])

    # create vpc
    vpc = create_vpc(conn, vpc_config["name"], vpc_config["vpc_cidr_block"], vpc_config["instance_tenancy"])
    subnet = create_subnet(conn, vpc, vpc_config["name"]+"-SN", vpc_config["subnet_cidr_block"])
    ig = create_ig(conn, vpc, vpc_config["name"]+"-IG")
    rt = update_route_table(conn, vpc, ig, vpc_config["name"]+"-RT")
    sg = create_sg(conn, vpc, sg_config["name"], sg_config["description"] + " for " + vpc_config["name"])
    add_rules_to_sg(conn, sg, sg_config["rules"])
    ins = create_instance(conn, vpc, sg, subnet, vpc_config["name"] + "-I", ins_config)
    #ec2.connection.associate_address('i-71b2f60b', None, 'eipalloc-35cf685d')


if __name__ == '__main__':
    main()