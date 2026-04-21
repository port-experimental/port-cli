# Session Context

## User Prompts

### Prompt 1

in a new branch, we want to expand the clear command to allow the deletion of: blueprints, entities, self service actions, pages, automations, scorecards. These are options the user can choose, from the entities, users and teams should not be cleared by default. This will be another flag

### Prompt 2

What about users and teams, are we preventing their deletion with the entities flag?

### Prompt 3

--blueprints should skip all blueprints starting with _ as they are system bluerpints that are protected. So yes

### Prompt 4

--delete-protected-blueprints will not work so it doesn't make sense, the only thing we can remove from them is the entities/actions/scorecards

### Prompt 5

open PR

### Prompt 6

Base directory for this skill: /Users/eric/.claude/plugins/cache/claude-plugins-official/superpowers/5.0.7/skills/finishing-a-development-branch

# Finishing a Development Branch

## Overview

Guide completion of development work by presenting clear options and handling chosen workflow.

**Core principle:** Verify tests → Present options → Execute choice → Clean up.

**Announce at start:** "I'm using the finishing-a-development-branch skill to complete this work."

## The Process

### Step...

### Prompt 7

2

