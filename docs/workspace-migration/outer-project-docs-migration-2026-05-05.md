# Outer Project Docs Migration - 2026-05-05

## Purpose

This note records the cleanup of agents_im-specific documentation that previously lived in the outer `/home/ws/project` workspace repository.

The current convention is:

- `/home/ws/project/agents_im` owns application-specific docs, product specs, design docs, exec plans, scripts, and CI/CD workflow notes.
- `/home/ws/project` remains a multi-project workspace/documentation repository and should only keep shared workspace notes.

## Migration result

The inner `agents_im` repository already contains the durable, newer versions of the project docs that matter for active development:

- `AGENTS.md`
- `ARCHITECTURE.md`
- `docs/GIT_WORKFLOW.md`
- `docs/AGENTIC_DEVELOPMENT_WORKFLOW.md`
- `docs/SECURITY.md`
- `docs/RELIABILITY.md`
- `docs/QUALITY_SCORE.md`
- `docs/PRODUCT_SENSE.md`
- `docs/FRONTEND.md`
- `docs/PLANS.md`
- `docs/product-specs/`
- `docs/design-docs/`
- `docs/exec-plans/`

The outer workspace had two historical active execution plans that are now represented by completed inner plans:

- outer `docs/exec-plans/active/message-service-contract.md` -> inner `docs/exec-plans/completed/message-service-contract.md`
- outer `docs/exec-plans/active/user-service-go-zero.md` -> inner `docs/exec-plans/completed/user-service-go-zero.md`

The outer workspace Git description has been copied here for reference:

- `docs/workspace-migration/outer-project-workspace-git.md`

## Follow-up for the outer repository

A separate outer-repo cleanup commit can remove or replace agents_im-specific files under `/home/ws/project/docs/` with pointers to this repository. That cleanup should be committed in the outer repository, not in the application repository.

Recommended outer repo policy after cleanup:

- keep workspace-wide notes such as `docs/PROJECT_WORKSPACE_GIT.md`;
- ignore nested app repos and worktrees;
- avoid adding new agents_im product/design/exec-plan content outside `/home/ws/project/agents_im`.
