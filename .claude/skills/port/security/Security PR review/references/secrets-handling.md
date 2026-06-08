# Secrets handling

- Store client credentials in vault or CI secret manager.
- Use `port auth login` locally; never commit `creds.json`.
- Rotate machine users after engineer offboarding.
- Redact tokens in logs and MCP debug output.
