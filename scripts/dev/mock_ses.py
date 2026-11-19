#!/usr/bin/env python3
"""Local mock of the Tencent SES SendEmail endpoint.

Used by `MOCK_SES=1 scripts/dev-up.sh` to exercise the full registration email
flow without cloud credentials. The app still goes through the real
auth-rpc -> resource-rpc -> SES-client code path; only the final HTTPS call is
captured here, and the verification code is written to <out>/requests.jsonl.

This is a DEV/TEST aid. It never validates the TC3 signature and must not be
exposed outside localhost.
"""

import argparse
import json
import os
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


class Handler(BaseHTTPRequestHandler):
    out_dir = ""

    def _write(self, payload):
        os.makedirs(self.out_dir, exist_ok=True)
        path = os.path.join(self.out_dir, "requests.jsonl")
        with open(path, "a", encoding="utf-8") as f:
            f.write(json.dumps(payload, ensure_ascii=False) + "\n")

    def do_GET(self):  # health probe
        if self.path == "/healthz":
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b"ok")
            return
        self.send_error(404)

    def do_POST(self):
        length = int(self.headers.get("Content-Length", 0))
        raw = self.rfile.read(length)
        try:
            payload = json.loads(raw or b"{}")
        except json.JSONDecodeError:
            payload = {"_raw": raw.decode("utf-8", "replace")}
        payload["_headers"] = {
            k: v for k, v in self.headers.items()
            if k.lower() in ("x-tc-action", "x-tc-version", "x-tc-region", "authorization")
        }
        self._write(payload)
        body = json.dumps({
            "Response": {
                "RequestId": "mock-request-id",
                "MessageId": "mock-message-id",
            }
        }).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, fmt, *args):  # keep the log quiet
        sys.stderr.write("mock-ses: " + (fmt % args) + "\n")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--port", type=int, default=9100)
    parser.add_argument("--out", required=True)
    args = parser.parse_args()
    Handler.out_dir = args.out
    server = ThreadingHTTPServer(("127.0.0.1", args.port), Handler)
    print(f"mock SES listening on 127.0.0.1:{args.port}, writing to {args.out}", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
