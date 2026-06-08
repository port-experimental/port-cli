# OWASP-oriented checks

| Risk | Look for |
|------|----------|
| Injection | Unsanitized blueprint filters in automations |
| Broken auth | Shared tokens across environments |
| Sensitive data | PII in skill `references/` committed to git |
| Misconfiguration | Public webhooks without signature verification |
