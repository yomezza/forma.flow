# Testing overview

Canonical detailed Go tests: go-testing.md. The minimum release gate is: (1) Go unit and table tests for pure scheduling and status transitions, (2) PostgreSQL integration for FK/GiST transactions and idempotency, (3) httptest HTTP validation, auth and error envelopes, (4) migration up/down in isolated environments, (5) multi-instance booking race tests, (6) frontend adapter contract checks against openapi.yaml.

Evidence for correctness: after N concurrent create attempts for same occupied interval, count distinct successful booking IDs is 1; no two rows matching the exclusion predicate overlap; a cancelled booking frees capacity; same idempotency request never creates a second booking; publication with a stale revision leaves both published and draft states untouched.

A test run without PostgreSQL is insufficient for release because mutex-based fakes do not enforce GiST exclusion or transaction ordering.
