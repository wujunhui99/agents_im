# CI/CD Success Rate Baseline

This document records the GitHub Actions CI/CD success-rate baseline before further workflow/process changes, so future changes can compare against a fixed snapshot.

## Snapshot

- Snapshot time: `2026-05-05T21:13:21+08:00`
- Repository: `wujunhui99/agents_im`
- Baseline commit on `main`: `9043b6dfea8a7db93f221ed7e269fae7e40483f5` (`9043b6d`)
- GitHub Actions runs fetched: `210`
- Completed workflow runs included in rate denominator: `210`
- Data range: `2026-04-29T08:06:13Z` to `2026-05-05T12:53:27Z`
- Source command: `gh api /repos/wujunhui99/agents_im/actions/runs --paginate --method GET -F per_page=100`

## Overall Success Rate

- Success rate: **182/210 = 86.67%**
- Denominator rule: completed workflow runs only. In-progress/queued runs are excluded. Non-success completed conclusions, including `failure`, `cancelled`, and `skipped`, count as not successful.

Conclusion counts:
- `cancelled`: `1`
- `failure`: `27`
- `success`: `182`

## Success Rate by Workflow

- CI: 138/152 = 90.79% (failure=14, success=138)
- Deploy to k3s: 44/58 = 75.86% (cancelled=1, failure=13, success=44)

## Key Branch Success Rates

- main: 106/132 = 80.30% (cancelled=1, failure=25, success=106)
- develop: 21/23 = 91.30% (failure=2, success=21)

Top other branches by completed run count:
- feature/im-delivery-correctness: 2/2 = 100.00% (success=2)
- feature/gateway-delivery-docs: 2/2 = 100.00% (success=2)
- feature/frontend-shell: 2/2 = 100.00% (success=2)
- feature/frontend-auto-load-regressions: 2/2 = 100.00% (success=2)
- feature/ws-live-push-e2e-regression: 1/1 = 100.00% (success=1)
- feature/transfer-gateway-dispatcher: 1/1 = 100.00% (success=1)
- feature/remove-frontend-mocks: 1/1 = 100.00% (success=1)
- feature/outbox-kafka-publisher: 1/1 = 100.00% (success=1)
- feature/mvp-social-group-rules: 1/1 = 100.00% (success=1)
- feature/mvp-reconnect-sync: 1/1 = 100.00% (success=1)

## Current CI/CD Run IDs at This Baseline

- `CI`: run `25377532452`, status `completed`, conclusion `success`, head `9043b6dfea8a7db93f221ed7e269fae7e40483f5`, created `2026-05-05T12:53:27Z`
  - URL: https://github.com/wujunhui99/agents_im/actions/runs/25377532452
- `Deploy to k3s`: run `25377532444`, status `completed`, conclusion `success`, head `9043b6dfea8a7db93f221ed7e269fae7e40483f5`, created `2026-05-05T12:53:27Z`
  - URL: https://github.com/wujunhui99/agents_im/actions/runs/25377532444

## Latest Runs Snapshot

- `25377532452` `CI` `completed/success` branch `main` head `9043b6d` created `2026-05-05T12:53:27Z`
- `25377532444` `Deploy to k3s` `completed/success` branch `main` head `9043b6d` created `2026-05-05T12:53:27Z`
- `25377280144` `CI` `completed/success` branch `develop` head `8258565` created `2026-05-05T12:48:10Z`
- `25374940105` `CI` `completed/success` branch `develop` head `8258565` created `2026-05-05T11:57:27Z`
- `25374724439` `CI` `completed/failure` branch `develop` head `1571743` created `2026-05-05T11:52:34Z`
- `25369016972` `Deploy to k3s` `completed/success` branch `main` head `89b5c2b` created `2026-05-05T09:40:31Z`
- `25369016952` `CI` `completed/success` branch `main` head `89b5c2b` created `2026-05-05T09:40:31Z`
- `25368120573` `Deploy to k3s` `completed/success` branch `main` head `8f61cca` created `2026-05-05T09:20:12Z`
- `25368120571` `CI` `completed/success` branch `main` head `8f61cca` created `2026-05-05T09:20:12Z`
- `25368106113` `Deploy to k3s` `completed/success` branch `main` head `b5d4cce` created `2026-05-05T09:19:52Z`
- `25368106109` `CI` `completed/success` branch `main` head `b5d4cce` created `2026-05-05T09:19:52Z`
- `25358465417` `CI` `completed/success` branch `main` head `06395d8` created `2026-05-05T04:50:45Z`

## Recent Non-success Runs

- `25374724439` `CI` `failure` branch `develop` head `1571743` created `2026-05-05T11:52:34Z` URL: https://github.com/wujunhui99/agents_im/actions/runs/25374724439
- `25358101582` `CI` `failure` branch `main` head `b4f6168` created `2026-05-05T04:37:10Z` URL: https://github.com/wujunhui99/agents_im/actions/runs/25358101582
- `25357161483` `Deploy to k3s` `failure` branch `main` head `34cbf07` created `2026-05-05T04:02:12Z` URL: https://github.com/wujunhui99/agents_im/actions/runs/25357161483
- `25284124460` `Deploy to k3s` `failure` branch `main` head `3e3079c` created `2026-05-03T16:09:51Z` URL: https://github.com/wujunhui99/agents_im/actions/runs/25284124460
- `25281722111` `CI` `failure` branch `develop` head `d7c003e` created `2026-05-03T14:24:56Z` URL: https://github.com/wujunhui99/agents_im/actions/runs/25281722111
- `25277482375` `CI` `failure` branch `main` head `03601b1` created `2026-05-03T11:06:51Z` URL: https://github.com/wujunhui99/agents_im/actions/runs/25277482375
- `25270735921` `Deploy to k3s` `failure` branch `main` head `2b60378` created `2026-05-03T05:19:22Z` URL: https://github.com/wujunhui99/agents_im/actions/runs/25270735921
- `25270735909` `CI` `failure` branch `main` head `2b60378` created `2026-05-03T05:19:22Z` URL: https://github.com/wujunhui99/agents_im/actions/runs/25270735909
- `25270702377` `CI` `failure` branch `main` head `580155a` created `2026-05-03T05:17:19Z` URL: https://github.com/wujunhui99/agents_im/actions/runs/25270702377
- `25270658150` `CI` `failure` branch `main` head `aab0804` created `2026-05-03T05:14:49Z` URL: https://github.com/wujunhui99/agents_im/actions/runs/25270658150
- `25270583579` `CI` `failure` branch `main` head `da5d059` created `2026-05-03T05:10:32Z` URL: https://github.com/wujunhui99/agents_im/actions/runs/25270583579
- `25270462034` `CI` `failure` branch `main` head `15d0e83` created `2026-05-03T05:03:58Z` URL: https://github.com/wujunhui99/agents_im/actions/runs/25270462034
- `25270316592` `CI` `failure` branch `main` head `4ce6a9f` created `2026-05-03T04:55:50Z` URL: https://github.com/wujunhui99/agents_im/actions/runs/25270316592
- `25250131568` `CI` `failure` branch `main` head `331af3a` created `2026-05-02T10:43:01Z` URL: https://github.com/wujunhui99/agents_im/actions/runs/25250131568
- `25249880859` `Deploy to k3s` `failure` branch `main` head `e93557f` created `2026-05-02T10:27:43Z` URL: https://github.com/wujunhui99/agents_im/actions/runs/25249880859
- `25243691552` `Deploy to k3s` `failure` branch `main` head `11cb5c6` created `2026-05-02T04:24:59Z` URL: https://github.com/wujunhui99/agents_im/actions/runs/25243691552
- `25243518770` `Deploy to k3s` `failure` branch `main` head `6649d5b` created `2026-05-02T04:15:00Z` URL: https://github.com/wujunhui99/agents_im/actions/runs/25243518770
- `25241091576` `Deploy to k3s` `failure` branch `main` head `deeed99` created `2026-05-02T02:04:25Z` URL: https://github.com/wujunhui99/agents_im/actions/runs/25241091576
- `25193698668` `Deploy to k3s` `failure` branch `main` head `6ddadcb` created `2026-04-30T23:05:18Z` URL: https://github.com/wujunhui99/agents_im/actions/runs/25193698668
- `25184783700` `Deploy to k3s` `failure` branch `main` head `18fd1e0` created `2026-04-30T19:20:30Z` URL: https://github.com/wujunhui99/agents_im/actions/runs/25184783700

## Notes for Future Comparisons

- Compare future snapshots using the same denominator rule unless intentionally changing the metric.
- For deployment quality, keep CI success rate separate from live k3s runtime verification; a green `Deploy to k3s` run means the workflow succeeded, not necessarily that every product E2E path passed.
- If GitHub retains more/less history in future API calls, record both total fetched runs and date range before comparing percentages.
