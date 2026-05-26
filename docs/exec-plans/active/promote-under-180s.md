# Promote-to-main under 180s plan

## Goal

Keep Drone `deploy-main` / promote-to-main under 180 seconds for normal scoped changes.
Build 169 took 323s: 216s image build plus 102s deploy. The largest avoidable costs were the shared backend seed build/cache export and overly broad `internal/` change selection.

## Changes in this branch

- Backend Docker image now compiles only `ARG SERVICE` instead of compiling all backend binaries in the first seed build.
- Drone backend image builds no longer do an expensive registry cache export on `main` by default; web cache export remains enabled.
- Backend cache refs are per service (`backend-<service>`) instead of one shared backend cache manifest.
- Build parallelism increased to 6 for deploy-main/devops image builds.
- Deploy detector maps domain-specific `internal/handler`, `internal/logic`, and `internal/servicecontext` paths to affected services instead of treating every `internal/` change as all backends.
- Deploy output skips the expensive `kubectl get svc,ingress` listing unless `AGENTS_IM_DEPLOY_LIST_RESOURCES=true`.

## Expected effect

For a route/API + web change similar to build 169, detector output should move from all 14 backend images + web to six API backends + web, excluding RPCs, gateway, mail-rpc, and message-transfer unless affected by domain logic.
Combined with no backend cache export, the critical path should drop well below 180s on warm builders.

## Validation

Run:

```bash
bash -n scripts/ci/drone-build-images.sh scripts/deploy-k3s.sh scripts/test-deploy-k3s.sh
python3 -m py_compile scripts/detect-deploy-changes.py
bash scripts/test-deploy-k3s.sh
git diff --check
```

Optional production proof requires merging through Merge Queue and checking the next Drone `deploy-main` duration.
