# INOS Mesh Delegation: Strategic Architecture

This document outlines the high-fidelity architecture for secure, efficient, and incentivized compute/storage delegation in the INOS mesh.

---

## 1. Core Philosophy
Delegation is not just about offloading work; it's about **fluid resource orchestration**. Tasks flow to where they are most efficient (latency, energy, cost) while maintaining **cryptographic integrity**.

---

## 2. Capability Architecture
Instead of simple strings, capabilities are advertised with structural performance and cost metrics.

### CapabilityAdvertisement (Tier 1)
| Component | Purpose |
|-----------|---------|
| `id` | Standard identifier (e.g., `hash.blake3.avx512`) |
| `score` | Normalized performance metric (0-1) |
| `costPlan` | Microcredits per MB/Cycle |
| `hardware` | Feature flags (AVX-512, GPU, TPM) |

---

## 3. Intelligent Decision Engine (`The Decider`)
The `DelegationEngine` acts as a cost-benefit analyzer for every job.

### Factors in `predictEfficiency()`
1. **Network Latency**: Cost of moving data vs. cost of local execution.
2. **Local Pressure**: Epoch delta and queue depth on local unit supervisors.
3. **Economic Yield**: Potential credit savings vs. delegation cost.
4. **Hardware Specifics**: Does a peer have AVX-512 for this BLAKE3 hash?

---

## 4. Secure Delegation: Zero-Trust Verification

### 4.1 Streaming Verification
For large transfers, we don't wait for completion. Verification is **progressive**:
- **Immediate Header Check**: Verify initial bytes match expected structure.
- **Random Sampling**: Peer-to-peer random sample checks during stream.
- **Final Digest**: Full cryptographic signature check.

### 4.2 Merkle Partial Delegation
For massive datasets (e.g., world models), we delegate **shards** with **Merkle Proofs**.
- Input: Merkle Root + Chunk Indices.
- Output: Result + Merkle Path (Proof of Integrity).

---

## 5. Economic Layer
Leveraging `ledger.capnp`, every delegated task is a **Micro-Contract**.

### Flow
1. **Bid**: Requester creates `Order` with `maxCredits`.
2. **Match**: Providers with matching capabilities and reputation accept.
3. **Escrow**: `CreditSupervisor` locks credits for the job.
4. **Settlement**: On verified `Result`, credits transfer via `poUWCompletion`.

---

## 6. Implementation Roadmap

### Phase 1: The Foundation (Complete)
- [x] Define `delegation.capnp` schema.
- [x] Implement the `CapabilityRegistry` extension in Go.
- [x] Basic logic for `Digest-First` data movement.

### Phase 2: Integrity & Economics
- [ ] `StreamingVerifier` implementation.
- [ ] `EconomicLedger` hooks for job payment.
- [ ] Reputation-based peer scoring.

---

## Observations
- **SAB is the Local Root**: Within a node, SAB remains the source of truth. Mesh transport is an extension of the linear space.
- **Epochs Synchronize Market**: Economy epochs (EconomyLoop) drive the settlement of many micro-transactions at once.
