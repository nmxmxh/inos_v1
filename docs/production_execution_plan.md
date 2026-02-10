# INOS Production Execution Plan (3-Phase Program)

## 1. Purpose

This document converts INOS from visionary architecture into an execution program with measurable outcomes.  
It is designed to answer one question every sprint:

**Are we increasing proven capability, or only increasing narrative complexity?**

## 2. Program North Star

Deliver INOS as a **credible, benchmarked, production-capable browser-native runtime** before expanding into speculative cognition features.

Priority order:

1. Runtime correctness and performance.
2. Mesh reliability and deterministic behavior.
3. Economic/security hardening.
4. AI infrastructure integration.
5. Experimental cognition research (separately labeled).

## 3. Program Rules (Non-Negotiable)

1. No "production-ready" claim unless phase exit gates are met.
2. No new moonshot tracks while a prior phase has failing critical gates.
3. Every phase ships:
   - Reproducible benchmark pack.
   - CI test matrix and pass thresholds.
   - Updated docs with "Implemented vs Experimental" status table.
4. Experimental AI work must be labeled `R&D` until validated by objective tests.

## 4. Timeline and Phases

## Phase 1 (Weeks 1-12): Core Credibility

Objective:

- Prove runtime fundamentals are stable, testable, and truthfully documented.

In scope:

- SAB/epoch runtime hardening.
- Go/Rust/JS integration reliability.
- Test determinism and CI pass discipline.
- Documentation truth-alignment.

Out of scope:

- "Distributed consciousness" and similar cognition claims.
- New large subsystems without passing runtime gates.

Deliverables:

1. Stable runtime release candidate.
2. Deterministic test suite baseline.
3. Public benchmark report template and first report.
4. Docs claim matrix for all major claims.

Mandatory technical tasks:

1. Fix deterministic failing mesh routing test.
2. Separate env-dependent network transport tests into tagged suites (`integration/network`), keep unit suite deterministic.
3. Replace explicit placeholders/stubs in critical paths or reclassify feature as experimental.
4. Add CI job tiers:
   - `fast` (PR gate)
   - `full` (merge gate)
   - `nightly` (stress/integration)
5. Add runtime SLO instrumentation:
   - Epoch propagation latency
   - Job dispatch latency
   - Worker startup failure rate

Exit gates (all required):

1. PR gate stability: >= 98% pass rate across 14 consecutive days.
2. Full suite: >= 95% pass rate across 10 consecutive runs.
3. Deterministic failure count in core runtime tests: 0.
4. Published benchmark artifact from clean environment (scripted, reproducible).
5. Spec/README claims reconciled with implemented status.

Go/No-Go:

- If any gate fails, Phase 2 is blocked.

---

## Phase 2 (Weeks 13-28): Mesh Reliability + Economic Hardening

Objective:

- Make the mesh and delegation stack dependable under realistic network conditions.

In scope:

- Gossip/DHT behavior correctness.
- Transport reliability and fallback quality.
- Delegation settlement integrity and abuse resistance.
- Operational observability.

Out of scope:

- New AI cognition protocols.
- Broad product expansion unrelated to mesh stability.

Deliverables:

1. Mesh Reliability Release (`MR-1`).
2. Delegation + economic settlement hardening report.
3. Fault-injection and chaos test results.

Mandatory technical tasks:

1. Define reliability SLOs:
   - Message delivery success under packet loss profiles.
   - Reconnection success time.
   - Delegation completion rate.
2. Harden transport:
   - Complete or isolate unimplemented receive path behavior.
   - Add explicit backoff and failure reason taxonomy.
3. DHT correctness:
   - Replace placeholder health/scoring heuristics with validated algorithms.
   - Add deterministic tests for lookup, replication, eviction, and churn.
4. Economic controls:
   - Anti-spam and abuse budgets.
   - Delegation payment/incentive consistency checks.
5. Observability:
   - Node health dashboard.
   - Mesh event tracing with correlation IDs.

Exit gates (all required):

1. Mesh integration tests pass in controlled environments with fault profiles.
2. Delegation success >= 99% in soak tests (defined workload).
3. No critical unresolved TODO/stub in mesh critical path.
4. Economic invariants hold across replay tests (no double-settlement, no negative balances).
5. Incident playbook and rollback procedure documented and tested.

Go/No-Go:

- If mesh SLO gates miss targets, do not progress to Phase 3.

---

## Phase 3 (Weeks 29-52): AI Infrastructure + Bounded R&D

Objective:

- Position INOS as AI runtime infrastructure with practical integrations, while containing speculative cognition work.

In scope:

- AI workload orchestration primitives.
- Practical ingestion/feature pipelines.
- Integration with mainstream model-serving/inference interfaces.
- R&D branch for Cerebral/FFI concepts with strict labeling.

Out of scope:

- Presenting R&D cognition concepts as shipped product.

Deliverables:

1. AI Infrastructure Release (`AIR-1`): stable orchestration substrate for AI workloads.
2. Integration adapters (at least 2) for existing model/inference ecosystems.
3. Separate `R&D-Cerebral` branch/docs with isolated acceptance criteria.

Mandatory technical tasks:

1. Implement missing video/data ingestion operations used for AI pipelines.
2. Standardize payload contracts and observability for inference jobs.
3. Add performance baselines:
   - Ingestion throughput
   - End-to-end inference pipeline latency
   - Resource efficiency per workload class
4. For Cerebral/FFI R&D:
   - Keep behind explicit feature flag.
   - Maintain separate validation matrix.
   - No merge to default product path without gate approval.

Exit gates (all required):

1. AI pipeline demos are reproducible from scripted setup.
2. Integration tests cover adapters and fallback behavior.
3. AI infrastructure SLO targets met for agreed workloads.
4. R&D cognition components remain clearly separated in docs and release notes.
5. No ambiguous claim language in public docs.

Go/No-Go:

- If adapters and pipeline reliability are weak, prioritize infrastructure hardening over cognition R&D.

## 5. Workstream Ownership Model

Use role ownership even if team is small:

1. Runtime Owner:
   - SAB/worker/kernel correctness, perf, and CI.
2. Mesh Owner:
   - transport, gossip, DHT, delegation reliability.
3. Economics/Security Owner:
   - invariants, abuse prevention, settlement consistency.
4. AI Infra Owner:
   - ingestion + orchestration + adapter interfaces.
5. Docs/Release Owner:
   - claim hygiene, release evidence, benchmark packs.

Single person teams still assign roles explicitly to avoid context collapse.

## 6. Weekly Operating Cadence

1. Monday:
   - Gate review (last week failures, current blockers).
2. Tuesday-Wednesday:
   - Deep implementation on critical-path issues only.
3. Thursday:
   - Benchmarks + integration runs.
4. Friday:
   - Claim audit, release notes, risk register update.

Hard rule:

- No new feature track starts on Friday.

## 7. Evidence Required Each Sprint

Every sprint must publish:

1. Test summary:
   - pass/fail by suite
   - flaky tests list
2. Performance summary:
   - trend vs prior sprint
3. Reliability summary:
   - SLO attainment
4. Documentation delta:
   - implemented
   - experimental
   - deferred

If evidence is missing, sprint is incomplete.

## 8. Risk Register (Top 8)

1. Narrative drift beyond implementation.
2. Test flakiness hiding real regressions.
3. Browser/platform variability impacting SAB behavior.
4. Mesh complexity outpacing observability.
5. Economic abuse vectors not modeled early.
6. Too many parallel moonshot tracks.
7. Incomplete critical path code hidden behind broad capability claims.
8. Burnout/context switching in small-team execution.

Mitigation policy:

- For each risk, assign owner + trigger + response within same sprint.

## 9. Documentation Governance (Claim Hygiene)

Maintain one authoritative table per release:

- `Implemented`
- `Experimental (feature-flagged)`
- `Roadmap`

Required updates per release:

1. `README.md`
2. `docs/spec.md`
3. `docs/ai.md`
4. Release notes

Any feature without test evidence cannot be marked `Implemented`.

## 10. Immediate 30-Day Action Plan

Week 1:

1. Establish CI tiers and deterministic test baseline.
2. Triage and fix known deterministic routing test failure.

Week 2:

1. Tag and isolate env-dependent transport tests.
2. Publish first benchmark script + output artifact template.

Week 3:

1. Replace/reclassify critical stubs in current release path.
2. Add docs claim matrix and status badges by maturity level.

Week 4:

1. Run phase gate rehearsal.
2. Publish `Phase 1 Gate Report` with pass/fail and blocker list.

## 11. Success Definition

INOS is successful when external technical reviewers can verify:

1. Claims match shipped behavior.
2. Runtime and mesh behavior are reproducible and measurable.
3. Reliability improves release over release.
4. AI positioning is practical and evidence-backed.
5. Experimental cognition work is clearly separated from production promises.

---

This plan is strict by design.  
If followed, INOS can convert strong architecture into durable execution credibility.
