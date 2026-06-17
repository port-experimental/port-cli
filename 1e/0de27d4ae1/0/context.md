# Session Context

## User Prompts

### Prompt 1

Do a PR review for this PR https://github.com/port-experimental/port-cli/pull/84

### Prompt 2

Base directory for this skill: /Users/eric/.claude/skills/review

<!-- AUTO-GENERATED from SKILL.md.tmpl — do not edit directly -->
<!-- Regenerate: bun run gen:skill-docs -->


## When to invoke this skill

Analyzes diff against the base branch for SQL safety, LLM trust
boundary violations, conditional side effects, and other structural issues. Use when
asked to "review this PR", "code review", "pre-landing review", or "check my diff".
Proactively suggest when the user is about to merge or la...

### Prompt 3

Add tests using the testing specialist

### Prompt 4

This session is being continued from a previous conversation that ran out of context. The summary below covers the earlier portion of the conversation.

Summary:
1. Primary Request and Intent:
   - User requested a PR review of https://github.com/port-experimental/port-cli/pull/84 via the `/review` skill
   - After review findings, user selected (via multi-select AskUserQuestion) three fixes to apply: export `userStatusForCreate`, remove dead `WorkerPool` parameter, unify `batchSize` constant
  ...

### Prompt 5

push to the PR

