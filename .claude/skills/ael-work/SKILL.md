---
name: ael-work
description: Continue authorized AEL project engineering work or an explicitly requested AEL task.
---

For an authorized AEL engineering request, run `./ael session show --session HOST_SESSION_ID --json` and read the returned frozen `session.skill_path`. Prepare a missing tool session with `./ael session start --session HOST_SESSION_ID --runner RUNNER --json`, then read its frozen Skill. After compaction or resume, first run `./ael recovery check --session HOST_SESSION_ID --json`. Ordinary discussion and review follow the user request directly.
