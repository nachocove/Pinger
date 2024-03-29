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
import boto.iam
from boto.s3.key import Key
from boto.vpc import VPCConnection
from boto.ec2.elb.attributes import AccessLogAttribute
import configparser
import StringIO
import pem
import subprocess
import uuid

# TODO : replace this
CREATE_NACHO_INIT_SH="../config/nacho_init.sh-template"

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
    conn = VPCConnection(region=region)
    vpc = get_vpc_by_name(conn, name)
    if not vpc:
        print "VPC %s does not exist. Nothing to delete" % name
    else:
        delete_sgs_for_vpc(conn, vpc, name)
        delete_route_tables_for_vpc(conn, vpc, name)
        delete_igs_for_vpc(conn, vpc, name)
        delete_subnets_for_vpc(conn, vpc, name)
        print "Deleting VPC %s..." % name
        success = False
        for i in range(0,100):
            try:
                conn.delete_vpc(vpc.id)
                success = True
                break
            except EC2ResponseError as ex:
                if ex.error_code == "DependencyViolation":
                    print "WARN(%s): Sleeping 2 and retrying" % ex.error_code
                    time.sleep(2)
                    continue
                else:
                    raise
        if not success:
            raise Exception("Could not delete vpc")

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

# delete internet gateways
def delete_igs_for_vpc(conn, vpc, name):
    ig_list = conn.get_all_internet_gateways(filters=[("attachment.vpc-id", vpc.id)])
    if not len(ig_list):
        print "No internet gateway exist for VPC %s. Nothing to delete" % name
    else:
        for ig in ig_list:
            print "Deleting internet gateway %s..." % ig.id
            conn.detach_internet_gateway(ig.id, vpc.id)
            conn.delete_internet_gateway(ig.id)

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
    # sometimes the subnet takes a bit to get created. try thrice
    subnet_list = []
    for x in range(0, 3):
        try:
            subnet_list = c.get_all_subnets(subnet_ids=[sn_id])
            break
        except EC2ResponseError:
            print "Waiting for subnet(%s) to be created" % sn_id
            time.sleep(1)
    while subnet_list[0].state == 'pending':
        print "waiting for subnet(%s) to be created" % sn_id
        time.sleep(1)
        subnet_list = c.get_all_subnets(subnet_ids =[sn_id])

# delete subnet
def delete_subnets_for_vpc(conn, vpc, name):
    subnet_list = conn.get_all_subnets(filters=[("vpcId", vpc.id)])
    if not len(subnet_list):
        print "No subnet exist for VPC %s. Nothing to delete" % name
    else:
        for sn in subnet_list:
            print "Deleting Subnet %s..." % sn.id
            conn.delete_subnet(sn.id)

# create subnet
def create_subnet(conn, vpc, name, cidr_block, availability_zone):
    print "Creating subnet %s" % name
    subnet_list = conn.get_all_subnets(filters=[("cidrBlock", cidr_block), ("vpcId", vpc.id),
                                                ("availabilityZone", [availability_zone])])
    if not len(subnet_list):
        subnet = conn.create_subnet(vpc.id, cidr_block, availability_zone=availability_zone)
        print "Created subnet %s (%s) for cidr_block: %s with available IPs: %d" % (name, subnet.id, subnet.cidr_block, subnet.available_ip_address_count)
        wait_for_subnet(conn, subnet.id)
        subnet.add_tag("Name", name)
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

# delete route table
def delete_route_tables_for_vpc(conn, vpc, name):
    rt_list = conn.get_all_route_tables(filters=[("vpc-id", vpc.id)])
    if not len(rt_list):
        print "No route tables exist for VPC %s. Nothing to delete" % name
    else:
        for rt in rt_list:
            print "Deleting Route table %s..." % rt.id
            for route in rt.routes:
                if route.gateway_id == 'local':
                    continue
                try:
                    conn.delete_route(rt.id, route.destination_cidr_block)
                except boto.exception.EC2ResponseError, e:
                    print "Error deleting route %s: %s" % (route.destination_cidr_block, e.error_message)
                    raise
            #conn.delete_route_table(rt.id)

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

# delete security group
def delete_sgs_for_vpc(conn, vpc, name):
    print "Deleting security groups for VPC %s" % name
    sg_list = conn.get_all_security_groups(filters=[("vpc-id", vpc.id)])
    for sg in sg_list:
        if sg.name == 'default' or ('Name' in sg.tags and sg.tags['Name'] == "default"):
            continue
        success = False
        for i in range(0,100):
            try:
                print "  Deleting VPC-SG %s" % sg.name
                conn.delete_security_group(group_id=sg.id)
                success = True
                break
            except EC2ResponseError as ex:
                if ex.error_code == "InvalidGroup.NotFound":
                    pass
                elif ex.error_code == "DependencyViolation":
                    # probably need to wait for the instance to die
                    print "WARN(%s): Retrying in 2 seconds." % ex.error_code
                    time.sleep(2)
                    continue
                raise
        if not success:
            raise Exception("Could not delete security group")

# create Security Group
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
def sg_rule_exists(sg, rule, is_ingress):
    if (is_ingress):
        rules = sg.rules
    else:
        rules = sg.rules_egress
    for r in rules:
        if r.ip_protocol == rule["protocol"] and  int(r.from_port) == rule["from_port"] and\
                int(r.to_port) == rule["to_port"] and r.grants[0].cidr_ip == rule["cidr_ip"]:
            return True
    return False

# load sg by id
def get_sg_by_id(conn, sgid):
    sg_list = conn.get_all_security_groups(group_ids=[sgid])
    if not len(sg_list):
        print "Security Group (%s) not found" % sgid
        return None
    else:
        print "Security Group (%s) found" % sgid
        return sg_list[0]

# delete all egress rules
def delete_egress_rules_from_sg(conn, sg):
    sg=get_sg_by_id(conn, sg.id)
    if sg:
        for rule in sg.rules_egress:
            print "Deleting egress rule ", rule
            conn.revoke_security_group_egress(sg.id, ip_protocol=rule.ip_protocol, from_port=rule.from_port,
                                              to_port=rule.to_port, cidr_ip=rule.grants[0])

# add rules to sg
def add_rules_to_sg(conn, sg, rules, is_ingress, jump_host):
    print "Adding rules to security group - IsIngress(%s)" % is_ingress
    if not is_ingress:
        if len(rules):
            delete_egress_rules_from_sg(conn, sg)
    for rule in rules:
        # reload sg again
        if rule['cidr_ip'] == "xx.xx.xx.xx/32":
            rule['cidr_ip'] = "%s/32" % jump_host

        sg = get_sg_by_id(conn, sg.id)
        if sg_rule_exists(sg, rule, is_ingress):
            print "Rule [(%s)-from_port-(%s)-to_port-(%s)-allow-access(%s) exists." % (rule["protocol"], rule["from_port"], rule["to_port"], rule["cidr_ip"])
        else:
            if is_ingress:
                sg.authorize(ip_protocol=rule["protocol"], from_port=rule["from_port"], to_port=rule["to_port"], cidr_ip=rule["cidr_ip"])
            else:
                conn.authorize_security_group_egress(sg.id, ip_protocol=rule["protocol"], from_port=rule["from_port"], to_port=rule["to_port"], cidr_ip=rule["cidr_ip"])
            print "Rule [(%s)-from_port-(%s)-to_port-(%s)-allow-access(%s) added." % (rule["protocol"], rule["from_port"], rule["to_port"], rule["cidr_ip"])

# delete auto scale and launch configuration
def delete_autoscaler(region_name, name):
    conn = boto.ec2.autoscale.connect_to_region(region_name)
    asg_list = conn.get_all_groups(names=[name])
    if not len(asg_list):
        print "Auto Scaler %s does not exist. Nothing to delete" % name
    else:
        print "Deleting auto scaler %s..." % name
        conn.delete_auto_scaling_group(name, force_delete=True)
        for asg in asg_list:
            if asg.instances:
                for instance in asg.instances:
                    print "  Terminating instance %s" % instance.instance_id
                    success = False
                    for i in range(0,100):
                        try:
                            conn.terminate_instance(instance.instance_id)
                            success = True
                            break
                        except BotoServerError as ex:
                            if ex.error_code == "ValidationError":
                                print "WARN(%s): sleeping 2 and trying again" % ex.error_code
                                time.sleep(2)
                                continue
                            else:
                                raise
                    if not success:
                        raise Exception("Could not terminate instance")

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
    conn = boto.ec2.autoscale.connect_to_region(region_name)
    asg_list = conn.get_all_groups(names=[name])
    if not len(asg_list):
        lc_name = name + "-LC"
        lc_list = conn.get_all_launch_configurations(names=[lc_name])
        if not len(lc_list):
            print "Creating Launch Configuration (%s)" % lc_name
            lc = LaunchConfiguration(name=lc_name, image_id=as_config["ami_id"],
                key_name=as_config["key_pair"],
                security_groups=[sg.id],
                user_data=as_config["user_data"],
                instance_type=as_config["instance_type"],
                instance_monitoring=as_config["instance_monitoring"],
                associate_public_ip_address=True
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
            vpc_zone_identifier = [subnet.id], health_check_type="ELB", health_check_period="300",
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
    conn = boto.ec2.elb.connect_to_region(region_name)
    try:
        elb_list = conn.get_all_load_balancers(load_balancer_names=[name])
    except BotoServerError, e: # ELB by the given name does not exist
        elb_list = []
    if not len(elb_list):
        print "Elastic Load Balancer %s does not exist. Nothing to delete" % name
    else:
        print "Deleting Elastic Load Balancer %s..." % name
        conn.delete_load_balancer(name)

# create load balancer
def create_elb(region_name, vpc, subnet, sg, name, config, cert):
    print "Creating elastic load balancer %s" % name
    conn = boto.ec2.elb.connect_to_region(region_name)
    try:
        elb_list = conn.get_all_load_balancers(load_balancer_names=[name])
    except BotoServerError, e: # ELB by the given name does not exist
        elb_list = []
    if not len(elb_list):
        ports = config["ports"]
        elb = conn.create_load_balancer(name, None, complex_listeners=ports, subnets=[subnet.id], security_groups=[sg.id])
        hc = HealthCheck(
            interval = config["health_check"]["interval"],
            healthy_threshold = config["health_check"]["healthy_threshold"],
            unhealthy_threshold = config["health_check"]["unhealthy_threshold"],
            target = config["health_check"]["target"] + config["alive-check-token"]
        )
        elb.configure_health_check(hc)
        pkp_name = "PublicKeyPolicy-%s-BackendCert" % elb.name
        conn.create_lb_policy(elb.name, pkp_name, "PublicKeyPolicyType", {"PublicKey": cert})
        besap_name = "BackendAuthPolicy-%s-BackendCert" % elb.name
        conn.create_lb_policy(elb.name, besap_name, "BackendServerAuthenticationPolicyType",
                                       {"PublicKeyPolicyName": pkp_name})
        conn.set_lb_policies_of_backend_server(elb.name, config["backend_port"], [besap_name])
        sp_name = "Sticky-%s" % elb.name
        conn.create_lb_cookie_stickiness_policy(None, elb.name, sp_name)
        conn.set_lb_policies_of_listener(elb.name, config["elb_port"], sp_name)
        access_log = AccessLogAttribute()
        access_log.enabled = True
        access_log.s3_bucket_name = config["accesslog_bucket_name"]
        access_log.s3_bucket_prefix = ''
        access_log.emit_interval = 5
        conn.modify_lb_attribute(elb.name, "AccessLog", access_log)
        print "Created Elastic Load Balancer (%s) for VPC(%s)" % (elb.name, vpc.id)
    else:
        elb = elb_list[0]
        print "Elastic Load Balancer (%s) found for VPC(%s)" % (elb.name, elb.vpc_id)
        if (elb.vpc_id != vpc.id):
            raise Exception("Error: Wrong VPC association: ELB(%s) is associated with VPC(%s) rather than VPC(%s)"
                            % (elb.name, elb.vpc_id, vpc.id))
    #elb.register_instances(ins.id)
    return elb

# create create_nacho_init_sh
def create_nacho_init_sh(config):
    M4 = subprocess.Popen("which m4", shell=True, stdout=subprocess.PIPE).stdout.readline().rstrip()
    if M4 == "":
        print "Error:m4 not found"
        config["as_config"]["user_data"]=""
    else:
        boot_user_name = config["vpc_config"]["name"] + "_pinger_boot"
        cw_user_name = config["vpc_config"]["name"] + "_pinger_cloudwatch"

        command = M4 + " -DACCESS_KEY=" + config["iam_config"][boot_user_name + "_access_key"]["access_key_id"] + \
        " -DSECRET_KEY=" + config["iam_config"][boot_user_name + "_access_key"]["secret_access_key"] + \
        " -DACCESS_KEY_CW=" + config["iam_config"][cw_user_name + "_access_key"]["access_key_id"] + \
        " -DSECRET_KEY_CW=" + config["iam_config"][cw_user_name + "_access_key"]["secret_access_key"] + \
        " -DBUCKET=" + config["s3_config"]["s3_bucket"] + " -DPREFIX=" + config["s3_config"]["bucket_prefix_key"] + \
        " -DPINGER_CFG=" + config["vpc_config"]["name"] + "-pingerv2.cfg" + \
        " -DLOG_GROUP_NAME=" + config["vpc_config"]["log_group_name"] + \
        " " + CREATE_NACHO_INIT_SH
        init_sh = subprocess.Popen(command, shell=True, stdout=subprocess.PIPE).stdout.read()
        config["autoscale_config"]["user_data"] = init_sh

# update pinger config
def update_pinger_cfg(config):
    user_name = config["vpc_config"]["name"] + "_pinger"
    config["s3_config"]["pinger_config"]["aws"]["accessKey"] = config["iam_config"][user_name + "_access_key"]["access_key_id"]
    config["s3_config"]["pinger_config"]["aws"]["secretKey"] = config["iam_config"][user_name + "_access_key"]["secret_access_key"]
    alive_token = uuid.uuid4().hex
    config["s3_config"]["pinger_config"]["server"]["alive-check-token"] = alive_token
    config["elb_config"]["alive-check-token"] = alive_token

# delete pinger cfg
def delete_pinger_cfg(vpc_name, s3_config):
    pinger_cfg_name = vpc_name + "-pingerv2.cfg"
    key_name = s3_config["bucket_prefix_key"] + "/" + s3_config["pinger_bucket_key"] + "/" + pinger_cfg_name
    conn = boto.connect_s3()
    bucket = boto.s3.bucket.Bucket(conn, s3_config["s3_bucket"])
    key = bucket.get_key(key_name)
    if key:
        print "Deleting pinger config (%s)" % key_name
        bucket.delete_key(key_name)
    else:
        print "Pinger Config %s not found to delete" % key_name

# upload pinger cfg
def upload_pinger_cfg(vpc_name, s3_config):
    pinger_cfg_name = vpc_name + "-pingerv2.cfg"
    print "Uploading pinger config (%s)" % pinger_cfg_name
    conn = boto.connect_s3()
    bucket = boto.s3.bucket.Bucket(conn, s3_config["s3_bucket"])
    pinger_config = s3_config["pinger_config"]
    buf = StringIO.StringIO()
    pinger_config.write(buf)
    new_key = Key(bucket)
    key_name = s3_config["bucket_prefix_key"] + "/" + s3_config["pinger_bucket_key"] + "/" + pinger_cfg_name
    new_key.key = key_name
    new_key.set_contents_from_string(buf.getvalue())

# delete iam users and policies
def delete_iam_users_and_policies(region_name, name_prefix, iam_config):
    print "Deleting IAM users and policies for %s" % name_prefix
    conn=boto.iam.connect_to_region(region_name)
    for user_config in iam_config["users"]:
        user_name = name_prefix + "_" + user_config
        try:
            user =  conn.get_user(user_name=user_name)
            policy_configs = iam_config["users"][user_config]["inline_policies"]
            for policy_config in policy_configs:
                policy_name = user_name + "_policy"
                try:
                    policy =  conn.get_user_policy(user_name, policy_name)
                except BotoServerError:
                    policy = None
                if policy:
                    print "Deleting policy (%s)" % policy_name
                    conn.delete_user_policy(user_name, policy_name)
            access_keys_list = conn.get_all_access_keys(user_name)['list_access_keys_response']['list_access_keys_result']['access_key_metadata']
            for access_key in access_keys_list:
                print "Deleting access_key (%s) for user (%s)" % (access_key["access_key_id"], user_name)
                conn.delete_access_key(access_key["access_key_id"], user_name=user_name)
            print "Deleting user (%s)" % user_name
            conn.delete_user(user_name)
        except BotoServerError:
            print "  User %s not found to delete" % user_name

# create iam users and policies
def create_iam_users_and_policies(region_name, name_prefix, iam_config):
    print "Creating IAM users and policies for %s" % name_prefix
    conn=boto.iam.connect_to_region(region_name)
    for user_config in iam_config["users"]:
        user_name = name_prefix + "_" + user_config
        try:
            user = conn.get_user(user_name=user_name)
        except BotoServerError:
            user = None
        if not user:
            print "Creating user (%s)" % user_name
            conn.create_user(user_name)
        else:
            print "User (%s) already exists." % user_name
        policy_configs = iam_config["users"][user_config]["inline_policies"]
        for policy_config in policy_configs:
            policy_name = user_name + "_policy"
            try:
                policy =  conn.get_user_policy(user_name, policy_name)
            except BotoServerError:
                policy = None
            if not policy:
                print "Creating policy (%s)" % policy_name
                conn.put_user_policy(user_name, policy_name, json.dumps(policy_config["policy"], indent=4))
            else:
                print "Updating policy (%s)" % policy_name
                conn.put_user_policy(user_name, policy_name, json.dumps(policy_config["policy"], indent=4))
        access_keys_list = conn.get_all_access_keys(user_name)['list_access_keys_response']['list_access_keys_result']['access_key_metadata']
        if not len(access_keys_list):
            print "Creating access key for user (%s)" % user_name
            key_response = conn.create_access_key(user_name)
            access_key = key_response.create_access_key_response.create_access_key_result.access_key
        else:
            access_key = access_keys_list[0]
            print "Access key exists for user (%s)" % user_name, access_key["access_key_id"]
            print "No 'secret_access_key' available. Please fill it in the init.sh in the user_data"
            access_key["secret_access_key"] = "Fill_ME_IN"
        iam_config[user_name + "_access_key"] = access_key

# cleanup
def cleanup(config):
    print "Cleaning up..."
    deprovision_pinger(config)

# process config
def process_config(config):
    config["aws_config"]["region"] = get_region(config["aws_config"]["region_name"])

# load config from S3
def load_config_from_s3(s3_config):
    conn = boto.connect_s3()
    bucket = boto.s3.bucket.Bucket(conn, s3_config["s3_bucket"])
    s3_files = dict()
    for file_key in s3_config["s3_filenames"]:
        file_name = s3_config["bucket_prefix_key"] + "/" + s3_config["pinger_bucket_key"] + "/" + s3_config["s3_filenames"][file_key]
        s3_key = Key(bucket, file_name)
        s3_files[file_key] = s3_key.get_contents_as_string()
    s3_config["s3_files"] = s3_files
    buf = StringIO.StringIO(s3_config["s3_files"]["pinger_config"])
    pinger_config = configparser.ConfigParser()
    pinger_config.read_file(buf)
    s3_config["pinger_config"] = pinger_config
    certs = pem.parse(s3_config["s3_files"]["certs_pem"])
    s3_config["certs"] = certs
    s3_config["first_cert_pem"] = str(certs[0])

# load json config
def json_config(file_name):
    with open(file_name) as data_file:
        json_data = json.load(data_file)
    #pprint json_data
    return json_data

# delete VPC et al
def deprovision_pinger(args):
    config = args.config
    process_config(config)
    aws_config = config["aws_config"]
    iam_config = config["iam_config"]
    s3_config = config["s3_config"]
    vpc_config = config["vpc_config"]
    if not args.name:
        print "ERROR: No vpc base-name provided."
        return

    print "De-Provisioning Pinger %s" % args.name
    delete_autoscaler(aws_config["region_name"], args.name + "-AS")
    delete_elb(aws_config["region_name"], args.name + "-ELB")
    delete_vpc(aws_config["region"], args.name)
    delete_pinger_cfg(args.name, s3_config)
    delete_iam_users_and_policies(aws_config["region_name"], args.name, iam_config)

# create VPC et al
def provision_pinger(args):
    config = args.config
    process_config(config)

    aws_config = config["aws_config"]
    iam_config = config["iam_config"]
    s3_config = config["s3_config"]
    vpc_config = config["vpc_config"]
    as_config = config["autoscale_config"]
    elb_config = config["elb_config"]
    misc_config = config["misc"]
    load_config_from_s3(s3_config)
    print "Provisioning Pinger %s" % vpc_config["name"]
    # create connection
    conn = VPCConnection(region=aws_config["region"])
    # create vpc
    try:
        create_iam_users_and_policies(aws_config["region_name"], vpc_config["name"], iam_config)
        update_pinger_cfg(config)
        upload_pinger_cfg(vpc_config["name"], s3_config)
        create_nacho_init_sh(config)
        vpc = create_vpc(conn, vpc_config["name"], vpc_config["vpc_cidr_block"], vpc_config["instance_tenancy"])
        subnet = create_subnet(conn, vpc, vpc_config["name"]+"-SN", vpc_config["subnet_cidr_block"],
                               vpc_config["availability_zone"])
        ig = create_ig(conn, vpc, vpc_config["name"]+"-IG")
        rt = update_route_table(conn, vpc, ig, vpc_config["name"]+"-RT")
        elb_sg_config = config["elb_config"]["sg_config"]
        elb_sg = create_sg(conn, vpc, vpc_config["name"]+elb_sg_config["name"]+"-SG", elb_sg_config["description"]
                           + " for " + vpc_config["name"])
        add_rules_to_sg(conn, elb_sg, elb_sg_config["ingress-rules"], True, misc_config["jumphost"])
        add_rules_to_sg(conn, elb_sg, elb_sg_config["egress-rules"], False, misc_config["jumphost"])
        elb = create_elb(aws_config["region_name"], vpc, subnet, elb_sg, vpc_config["name"] + "-ELB",
                         elb_config, s3_config["first_cert_pem"])
        ins_sg_config = config["autoscale_config"]["sg_config"]
        ins_sg = create_sg(conn, vpc, vpc_config["name"]+ins_sg_config["name"]+"-SG",
                           ins_sg_config["description"] + " for " + vpc_config["name"])
        add_rules_to_sg(conn, ins_sg, ins_sg_config["ingress-rules"], True, misc_config["jumphost"])
        add_rules_to_sg(conn, ins_sg, ins_sg_config["egress-rules"], False, misc_config["jumphost"])
        ascaler = create_autoscaler(aws_config["region_name"], vpc, elb, subnet,
                                    ins_sg, vpc_config["name"] + "-AS", aws_config, as_config)
    except (BotoServerError, S3ResponseError, EC2ResponseError) as e:
        print "Error :%s(%s):%s" % (e.error_code, e.status, e.message)
        print traceback.format_exc()
        cleanup(config)

# main
def main():
    parser = argparse.ArgumentParser(description='Provision the Pinger at AWS')
    subparser = parser.add_subparsers()

    create_parser = subparser.add_parser("create")
    create_parser.add_argument('--config', required=True, type=json_config, metavar="config_file",
                   help='the config(json) file for the deployment', )
    create_parser.set_defaults(func=provision_pinger)

    delete_parser = subparser.add_parser("delete")
    delete_parser.add_argument('name', help="override cluster name", nargs='?', type=str, default=None)
    delete_parser.add_argument('--config', required=True, type=json_config, metavar="config_file",
                   help='the config(json) file for the deployment', )
    delete_parser.set_defaults(func=deprovision_pinger)

    args = parser.parse_args()
    args.func(args)

if __name__ == '__main__':
    main()
