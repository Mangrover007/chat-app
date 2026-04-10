# Contributing

Thanks for your interest in contributing.

---

## Getting Started

Before contributing, set up the project by following the instructions in [Readme.md](Readme.md).

---

## Workflow

- Pick or create an issue to work on
- Create a new branch for your work
- Open a **separate Pull Request (PR) for each issue**

Do not bundle multiple issues into a single PR.

---

## Pull Request Guidelines

Each PR should:

- Address exactly one issue
- Have a clear title
- Include a brief description of what was changed and why

Suggested PR title format:

```
[feat] add websocket reconnect handling
[fix] resolve race condition in consumer
[docs] update setup instructions
```

---

## Commit Guidelines

Your commit history should be **clean and readable**.

Follow this format for every commit:

```
[type] short description
```

Examples:

```
[feat] add message queue consumer
[fix] handle nil pointer in handler
[refactor] simplify connection pool logic
[docs] update README setup section
```

Common types:
- feat: new feature
- fix: bug fix
- refactor: code changes without behavior change
- docs: documentation updates
- chore: misc changes (build, config, etc.)

Avoid:
- vague messages like "update", "fix stuff"
- large commits covering unrelated changes

---

## Creating Issues

If you are creating a new issue, ensure it is **clear and meaningful**.

Each issue should include:

- A concise title
- A clear description of the problem or feature
- Most importantly, **the reason why this issue exists**

Explain:
- What problem it solves
- Why it matters
- Any relevant context

Example:

```
Problem:
WebSocket connections drop silently under load

Reason:
This causes clients to stop receiving messages without retrying, leading to data inconsistency

Proposed direction:
Add reconnect + heartbeat mechanism

Explanation:
- Heartbeats allow the server and client to detect dead connections
- Reconnect logic ensures the client automatically restores the connection
- Together, this improves reliability and prevents silent message loss
```

Avoid:
- One-line issues with no context
- Vague descriptions like "fix bug" or "improve performance"

---

## General Notes

- Keep changes minimal and focused
- Write code that is consistent with the existing codebase
- Test your changes before submitting a PR

---
