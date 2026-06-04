# MCP streaming

Some MCP clients require:

```
Accept: application/json, text/event-stream
```

Without `text/event-stream`, tool listing or `load_skill` may fail intermittently.

Check mcp-service logs for auth and org context (`x-port-user-orgid`).
