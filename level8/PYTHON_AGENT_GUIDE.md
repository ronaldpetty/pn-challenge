# Python MCP Agent Guide

This demo ships Go-based MCP agents, but a new agent can be written in Python as long as it exposes the same small HTTP surface:

- `GET /healthz`
- `GET /mcp/tools/list`
- `POST /mcp/tools/call`

## Minimal Python Agent

```python
#!/usr/bin/env python3
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os

AGENT_ID = os.environ.get("AGENT_ID", "python-echo")
TOOL = os.environ.get("TOOL", "echo")


class Handler(BaseHTTPRequestHandler):
    def _json(self, status, payload):
        body = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.path == "/healthz":
            self._json(200, {"ok": True, "service": AGENT_ID})
            return
        if self.path == "/mcp/tools/list":
            self._json(200, {
                "agent": AGENT_ID,
                "tools": [{"name": TOOL, "description": "Python demo tool"}],
            })
            return
        self._json(404, {"error": "not found"})

    def do_POST(self):
        if self.path != "/mcp/tools/call":
            self._json(404, {"error": "not found"})
            return
        length = int(self.headers.get("Content-Length", "0"))
        request = json.loads(self.rfile.read(length) or b"{}")
        if request.get("tool") != TOOL:
            self._json(400, {
                "agent": AGENT_ID,
                "requested_tool": request.get("tool"),
                "allowed_tools": [TOOL],
                "error": "skill mismatch",
            })
            return
        self._json(200, {
            "agent": AGENT_ID,
            "tool": TOOL,
            "input": request.get("input", ""),
            "result": request.get("input", ""),
        })

    def log_message(self, _format, *_args):
        return


if __name__ == "__main__":
    port = int(os.environ.get("PORT", "8080"))
    ThreadingHTTPServer(("", port), Handler).serve_forever()
```

## Add It To The Demo

1. Add a service to `compose.yaml` using `python:3.12-slim` or your own image.
2. Attach it to the enterprise network that should reach it, such as `enterprise_a_net`.
3. Add the agent to the relevant enterprise's `Agents` list in `internal/shared/shared.go`.
4. Set the endpoint to the compose service name, for example `http://enterprise-a-python-echo:8080`.
5. Run `./scripts/test-e2e.sh`.

## Expected Contract

The consumer will reject the agent unless:

- the signed catalog lists the tool name
- `/mcp/tools/list` returns the same tool name
- `/mcp/tools/call` rejects mismatched tools
- the service is reachable from the consumer's Docker network
