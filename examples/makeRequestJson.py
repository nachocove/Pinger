# Copyright 2014, NachoCove, Inc
import json
import base64
import argparse

def main(args):
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
                   "PushToken": args.push_token,
                   "PushService": args.push_service,
                   "ClientId": args.client_id,
                   "Platform": args.platform,
                   "HttpHeaders": headers,
                   "HttpRequestData": base64.b64encode(args.request_data.read()) if args.request_data else "",
                   "HttpExpectedReply": base64.b64encode(args.reply_data.read()) if args.reply_data else "",
                   "HttpNoChangeReply": base64.b64encode(args.no_change_reply.read()) if args.no_change_reply else "",
                   }
    #import urllib
    #return urllib.quote_plus(json.dumps(jsonPayload))
    return json.dumps(jsonPayload)

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument('--username', help="The username")
    parser.add_argument('--password', help="the password")
    parser.add_argument('--url', help="The host URL")
    parser.add_argument('--wait-before-use', type=int, default=30, help='Time (in seconds) to wait before starting to poll')
    parser.add_argument('--timeout', type=int, default=600, help='Time (in seconds) to wait for a response')
    parser.add_argument('--push-token', help='The Push Token (platform dependent)')
    parser.add_argument('--push-service', choices=('APNS',), help='The push service')
    parser.add_argument('--request-data', type=file, help='The request payload (file)')
    parser.add_argument('--reply-data', type=file, help='The reply payload (file)')
    parser.add_argument('--no-change-reply', type=file, help='The no-change reply (file)')
    parser.add_argument('--platform', choices=('ios',), help='The platform')
    parser.add_argument('--client-id', help='A client ID (cognito ID)')
    parser.add_argument('--header', action='append', default=[], help="Headers (format: Key=Value). Can be given multiple times.")

    args = parser.parse_args()
    print main(args)