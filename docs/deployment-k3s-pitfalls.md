# k3s Deployment Pitfalls and Runbook

This document records deployment/k3s issues that have already happened in this project and the checks to run before claiming a release is healthy.

## Scope

- Repository: `agents_im`
- Kubernetes namespace: `agents-im`
- Production deploy path: GitHub Actions `Deploy to k3s` workflow plus `scripts/deploy-k3s.sh`
- Secrets and server connection details must never be copied into docs or chat. Use `[REDACTED]` for DSNs, tokens, hosts, credentials, keys, and connection strings.

## Release verification checklist

Before reporting success, verify all three layers:

1. GitHub Actions
   - Check the exact `main` commit SHA.
   - Check both `CI` and `Deploy to k3s` runs for that SHA.
   - If a run fails or hangs, inspect the job/step logs; do not infer success from local tests.
2. k3s runtime
   - Run `kubectl -n agents-im get deploy,pods -o wide`.
   - Every deployment should show `READY 1/1` and every pod should be `Running`.
   - For failed rollouts, inspect logs for the affected deployment, including `--previous` for CrashLoopBackOff.
3. Application/E2E
   - Use the real public URL and real API paths.
   - If PostgreSQL was cleared, old test accounts and friendships may not exist; recreate test data through supported flows before E2E.

## Pitfall: GitHub Actions green is not enough

A deploy workflow can complete image build/apply steps while runtime pods still fail or remain unavailable. Always check k3s after Actions:

```bash
kubectl -n agents-im get deploy,pods -o wide
kubectl -n agents-im rollout status deploy/<service> --timeout=180s
kubectl -n agents-im logs deploy/<service> --tail=120 --previous || kubectl -n agents-im logs deploy/<service> --tail=120
```

Common signs:

- `CrashLoopBackOff`
- `0/1` READY
- rollout timeout
- old and new pods mixed during rollout

## Pitfall: protobuf descriptor conflicts can crash only at runtime

Observed failure:

```text
panic: proto: file "auth.proto" is already registered
See https://protobuf.dev/reference/go/faq#namespace-conflict
```

Impact:

- `auth-rpc` entered `CrashLoopBackOff` even though local package tests could pass.
- Reverting the feature commit did not immediately remove the failure until the runtime deployment was restarted/updated.

Root cause pattern:

- Generated protobuf descriptors used inconsistent source names such as `auth.proto`/`user.proto` while other generated files use `proto/<service>.proto`.
- Duplicate generated descriptors or mixed generation paths can register the same proto file name more than once in one binary.

Immediate mitigation used:

```bash
kubectl -n agents-im set env deployment/auth-rpc GOLANG_PROTOBUF_REGISTRATION_CONFLICT=warn
kubectl -n agents-im rollout status deploy/auth-rpc --timeout=180s
```

Repository-side compatibility guard:

- `deploy/k8s/deployments.yaml` sets `GOLANG_PROTOBUF_REGISTRATION_CONFLICT=warn` for `auth-rpc` until the protobuf generation paths are normalized.

Long-term fix:

- Regenerate all protobuf artifacts consistently from repository root with stable paths and `go_package` values.
- Avoid committing duplicate generated `.pb.go` outputs for the same proto service.
- Add a runtime import/boot smoke test for RPC binaries that can catch init-time protobuf panics before deployment.

## Pitfall: selective deploy can revert non-selected images

`kubectl apply -k` reapplies manifests that contain default `:latest` image tags. If only one service is selected for a new image, unselected services can be accidentally reset unless their current images are restored.

Current mitigation:

- `scripts/deploy-k3s.sh` records current images before apply.
- It sets new images only for selected services.
- It restores pre-apply images for unselected services.
- `scripts/test-deploy-k3s.sh` has regression coverage for web-only and config-only deploys.

Verification:

```bash
bash scripts/test-deploy-k3s.sh
```

## Pitfall: config-only deploys still restart services

Config-only changes can still affect runtime because ConfigMaps/Secrets and deployment templates roll pods. Treat config-only deploys as real releases:

- inspect rollout status for affected services;
- inspect logs if readiness does not become green;
- verify the deployed images did not unintentionally change.

## Pitfall: MailRPC endpoint lists must stay list-shaped

`auth-api` owns `POST /auth/register/email-code` and builds its Mail RPC client from `MailRPC.Endpoints`. The endpoint value must be a YAML list in both local examples and k3s configs:

```yaml
MailRPC:
  Endpoints:
    - <mail-rpc-service-endpoint>
```

Do not use scalar syntax such as `Endpoints: <mail-rpc-service-endpoint>`. The static verification gate rejects scalar `MailRPC.Endpoints` in auth configs so the email-code path does not boot without a configured mail client.

## Pitfall: hostNetwork concentrates failures on one node

Most backend deployments use `hostNetwork: true`. This makes port conflicts and node-level networking issues more likely than in ordinary pod networking.

Checks:

```bash
kubectl -n agents-im get deploy,pods -o wide
kubectl -n agents-im describe pod <pod>
```

Look for:

- bind/listen failures;
- readiness probes failing because the service did not bind the expected host port;
- several services scheduled to the same node with conflicting ports.

## Pitfall: PostgreSQL destructive resets invalidate E2E assumptions

When PostgreSQL is cleared, old accounts, profiles, auth credentials, friendships, groups, messages, and media metadata are gone. Do not reuse prior browser sessions or assume old test accounts still exist.

After a reset:

1. confirm schema exists;
2. recreate test accounts using supported auth/register or seed scripts;
3. recreate friendships/conversations if the UI flow requires them;
4. run E2E from a fresh browser/session state.

## PostgreSQL schema introspection commands

Use these on the server without printing credentials:

```bash
docker exec agents-im-postgres sh -lc 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "\dt public.*"'
docker exec agents-im-postgres sh -lc 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "SELECT table_name,column_name,data_type,is_nullable,column_default FROM information_schema.columns WHERE table_schema='"'"'public'"'"' ORDER BY table_name,ordinal_position"'
```

Do not paste raw environment variables or DSNs into reports.

## Standard failure triage order

1. Identify latest main SHA and corresponding Actions runs.
2. If CI failed, inspect CI logs first.
3. If deploy failed or hangs, inspect deploy logs and k3s state.
4. For `CrashLoopBackOff`, read `--previous` logs before restarting again.
5. Apply the smallest mitigation needed to restore service.
6. Commit durable manifest/script/doc changes so manual fixes are not lost on the next deploy.
7. Re-run local verification and monitor a fresh deploy.
