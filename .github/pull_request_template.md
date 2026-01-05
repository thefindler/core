## Required:
- [ ] Change the PR branch name to a clear, descriptive format (e.g. `feature/...`, `fix/...`, `chore/...`).
- [ ] In Cursor, start a message with `@Branch` manually and run the prompt below. Paste it by removing the following prompt.

```text
@Branch

You are a senior engineer doing a deep pre-merge quality sweep.

Instructions:
- Think thoroughly about the entire codebase and how these PR changes interact with it (not just the touched files).
- Identify edge cases, regressions, and realistic bug scenarios introduced by these changes.
- Apply best practices for reliability, security/privacy, performance, and maintainability.
- Be strict: add “confirm/verify” items where information is missing—don’t assume.

Output:
Return a PR-specific checklist (Markdown checkboxes). Keep it practical and action-oriented.

Must include sections (when relevant):
1) Functional correctness & edge cases
2) Error handling, timeouts, retries, idempotency
3) Logging/monitoring/alerts + dashboards to check
4) Security/privacy/compliance considerations
5) Performance & scalability concerns
6) Backward compatibility & migrations
7) Rollout plan + rollback plan

Also include these mandatory checklist items:
- Create a Loom video demonstrating the end-to-end functionality affected by this PR (happy path + at least 1 failure/edge case).
- After merge + deployment, re-check the changes in production and confirm expected behavior (include what you checked and where).

Now generate the checklist.

