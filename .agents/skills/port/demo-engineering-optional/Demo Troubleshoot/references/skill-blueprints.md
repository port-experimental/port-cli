# Skill blueprint graph

```
_skill_group ← skill_to_skill_group ← _skill
                                      ↑
                        skill_version_to_skill
                                      |
                               _skill_version
                                      ↑
                      skill_file_to_skill_version
                                      |
                                 _skill_file
```

Forward relations are canonical for search. Entity payloads may also expose target blueprint keys (`_skill`, `_skill_version`) when relations are included.

Legacy single blueprint: `skill` with `instructions`, `references[]`, `assets[]` on one entity.
