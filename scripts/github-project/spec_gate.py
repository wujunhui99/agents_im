#!/usr/bin/env python3
"""Lightweight Spec Gate for GitHub issue bodies.

Usage:
  gh issue view <number> --json body --jq .body | python3 scripts/github-project/spec_gate.py

The script validates section presence and a minimum acceptance-checklist shape.
It is intentionally text-based so it also works with Issue Forms rendered bodies.
"""
from __future__ import annotations

import re
import sys

REQUIRED_PATTERNS = {
    "Background": r"(?im)^#{1,3}\s*Background\b|^###?\s*Background\b|^Background\s*$",
    "Acceptance Criteria": r"(?im)^#{1,3}\s*Acceptance Criteria\b|^###?\s*Acceptance Criteria\b|^Acceptance Criteria\s*$",
    "Test Plan": r"(?im)^#{1,3}\s*Test Plan\b|^###?\s*Test Plan\b|^Test Plan\s*$",
}

PRODUCT_HINTS = [
    "User Story", "Functional Scope", "Interaction Flow", "Product Usability Requirements", "Edge Cases"
]


def main() -> int:
    body = sys.stdin.read()
    if not body.strip():
        print("FAIL: empty issue body")
        return 1
    failures: list[str] = []
    for name, pattern in REQUIRED_PATTERNS.items():
        if not re.search(pattern, body):
            failures.append(f"missing required section: {name}")
    for name in PRODUCT_HINTS:
        if name.lower() not in body.lower():
            failures.append(f"missing product-spec signal: {name}")
    checks = re.findall(r"(?m)^\s*- \[[ xX]\] ", body)
    if len(checks) < 5:
        failures.append(f"acceptance criteria should contain at least 5 checkbox items; found {len(checks)}")
    if "fake success" not in body.lower() and "失败" not in body and "error" not in body.lower():
        failures.append("issue should mention visible failure/error handling for product usability")
    if failures:
        print("FAIL: Spec Gate did not pass")
        for f in failures:
            print(f"- {f}")
        return 1
    print("PASS: Spec Gate passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
