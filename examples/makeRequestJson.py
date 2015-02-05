# Copyright 2014, NachoCove, Inc
import json
import base64
import argparse
import StringIO

def register_payload(args):
    credentials = {"Username": args.username,
                   "Password": args.password,
                   }

    headers = {}
    for header in args.header:
        k,v = header.split('=')
        headers[k] = v

    jsonPayload = {"WaitBeforeUse": args.wait_before_use,
                   "ResponseTimeout": args.timeout,
                   "MailServerUrl": args.url,
                   "MailServerCredentials": json.dumps(credentials),
                   "Protocol": args.protocol,
                   "PushToken": args.push_token,
                   "PushService": args.push_service,
                   "ClientId": args.client_id,
                   "Platform": args.platform,
                   "HttpHeaders": headers,
                   "HttpRequestData": base64.b64encode(args.request_data.read()) if args.request_data else "",
                   "HttpExpectedReply": base64.b64encode(args.reply_data.read()) if args.reply_data else "",
                   "HttpNoChangeReply": base64.b64encode(args.no_change_reply.read()) if args.no_change_reply else "",
                   "CommandTerminator": base64.b64encode(args.cmdTerminator.read()) if args.cmdTerminator else "",
                   "CommandAcknowledgement": base64.b64encode(args.cmdAck.read()) if args.cmdAck else "",
                   }
    #import urllib
    #return urllib.quote_plus(json.dumps(jsonPayload))
    return json.dumps(jsonPayload)

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--client-id', help='A client ID (cognito ID)')
    parser.add_argument('--url', help="The host URL")
    parser.add_argument('--username', help="The username")
    parser.add_argument('--password', help="the password")
    parser.add_argument('--protocol', choices=('ActiveSync',), help="The email protocols")
    parser.add_argument('--push-token', help='The Push Token (platform dependent)')
    parser.add_argument('--push-service', choices=('APNS',), help='The push service')    
    parser.add_argument('--platform', choices=('ios',), help='The platform')
    parser.add_argument('--header', action='append', default=[], help="Headers (format: Key=Value). Can be given multiple times.")
    parser.add_argument('--request-data', type=file, help='The request payload (file)')
    parser.add_argument('--reply-data', type=file, help='The reply payload (file)')
    parser.add_argument('--no-change-reply', type=file, help='The no-change reply (file)')
    parser.add_argument('--cmdTerminator', type=file, help="File containing the command terminator. Assumed to be binary")
    parser.add_argument('--cmdAck', type=file, help="File containing the command Acknowledgement. Assumed to be binary")
    parser.add_argument('--timeout', type=int, default=600, help='Time (in seconds) to wait for a response')
    parser.add_argument('--wait-before-use', type=int, default=30, help='Time (in seconds) to wait before starting to poll')

    parser.add_argument('--register-example', choices=('ActiveSync',), help="Create and print out an example payload for register")
    parser.add_argument('--defer-example', type=str, help="Create and print out an example payload defer. Argument is the client ID")

    args = parser.parse_args()
    if args.register_example:
        if args.register_example == 'ActiveSync':
            args.client_id = "us-east-1:0005d365-c8ea-470f-8a61-a7f44f145efb"
            args.url = "https://d2.officeburrito.com/Microsoft-Server-ActiveSync?Cmd=Ping&DeviceId=Ncho12345"
            args.username = "D2\test"
            args.password = "Password1"
            args.protocol = 'ActiveSync'
            args.push_token = "BEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEE"
            args.push_service = "APNS"
            args.platform = 'ios'
            args.header = ["MS-ASProtocolVersion=14.1",]
            args.request_data = StringIO.StringIO("You got any mail, man?")
            args.reply_data = StringIO.StringIO("Yep!")
            args.no_change_reply = StringIO.StringIO("Nah, man.")
            args.cmdTerminator = None
            args.cmdAck = None
            args.timeout = 0
            args.wait_before_use = 10
            print register_payload(args)
    elif args.defer_example:
        print json.dumps({'ClientId': args.defer_example})
    else:
        print register_payload(args)
    
if __name__ == "__main__":
    main()