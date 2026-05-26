# Local-only secrets

This directory is for operator-local convenience only. Real values in this
directory are intentionally ignored by Git and must never be committed, pasted in
issues/PRs, or printed in logs.

Use one independent file per secret or credential hint. Suggested local files:

- `drone_token` — local Drone API token for querying `https://drone.agenticim.xyz`.
- `server_ssh` — local SSH alias/login note, for example the configured SSH alias and username.
- `k8s_access` — local note that Kubernetes is accessed through the server SSH session, plus safe commands.

Security rules:

1. If a real token/password/private key is accidentally pasted into chat, logs, or
   Git, rotate it immediately.
2. Do not store provider API keys, Drone tokens, JWT secrets, database URLs,
   server hosts/users/ports, or private keys in tracked files.
3. Keep real secrets in their authoritative stores: Drone repository secrets,
   server files with restrictive permissions, and Kubernetes Secrets.
4. Use the `*.example` files here only as placeholders/templates.
