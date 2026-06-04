# Conditions and outlets

| Pattern | Example |
|---------|---------|
| Property equals | `properties.tier == "production"` |
| Relation exists | related `service` present |
| Scorecard level | bronze / silver / gold gates |

Prefer explicit outlets over nested if-chains for maintainability.

Demo seed assigns this skill to the **demo-engineering-optional** group; sync is controlled by `port skills` selection in the CLI.
