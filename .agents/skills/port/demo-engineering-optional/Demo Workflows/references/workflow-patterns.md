# Workflow patterns

## Entity-triggered

- **On create**: notify owner team, open ticket.
- **On property change**: recalculate scorecard, sync external system.

## Scheduled

- Nightly drift detection.
- Weekly compliance export.

## Human-in-the-loop

- Approval step before destructive API calls.
- Timeout → escalate to `references/conditions.md` fallback branch.
