# AGENTS.md — Rules for all agents

These rules are mandatory in every session when working on this project.

---

## 1. Temporary files

All temporary files (temp, scratch, build artifacts, debug logs, dumps, etc.) must be created **only** inside the project working directory.

## 2. Search for existing libraries

Before writing any new code (function, module, component, utility), you must:

1. Search the internet (npm, Go packages, PyPI, GitHub, etc.) — does a library already exist that solves this problem?
2. If a library is found — use it (unless there are strong reasons not to).
3. If no library is found, or you decide not to use one — briefly document the reason in code or in the PR/commit description.

## 3. Documenting code changes

Every code change must be documented. Minimum requirements:

- **Commits**: meaningful message reflecting the essence of the changes.
- **Code**: comments for non-obvious decisions, complex logic, choice of a specific library/approach.
- **PR/branch**: description of what was done, why, and what alternatives were considered.

## 4. Codemap after backend changes

After every change to backend code (directories `internal/`, `cmd/`, `runtime/`, `migrations/`):

1. Load the `codemap` skill.
2. Generate an up-to-date code map.
3. Save it to `CODEMAP.md` in the project root (overwrite the previous version).

Example:

```
Load the codemap skill, generate the backend code map, and save it to CODEMAP.md.
```
