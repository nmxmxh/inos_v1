# INOS Architectural Security & Integrity Audit (v1.9)

This document provides a technical deep-dive into the systemic vulnerabilities identified in the INOS v1.9 architecture. It serves as both an educational guide and a production-grade action plan for hardening the "Internet-Native Operating System."

---

## 1. Node Attestation: The Authenticity Gap
**The Issue**: There is currently no cryptographic or hardware verification that a peer joining the mesh is running the authentic INOS kernel.
**Educational Context**: In a decentralized mesh, "soft" verification (checking a binary hash) is useless if the attacker controls the reporting environment. An attacker can claim to be running INOS v1.9 while running a modified "Leech Kernel" that steals credits.
**Production-Grade Solution**:
- **Short-term (Handshake Challenges)**: Implement a "Challenge-Response" handshake where the node must compute a non-trivial result from signed, non-deterministic portions of its own linear memory (SAB).
- **Long-term (Trusted Execution)**:
    - **Native**: Utilize TPM (Trusted Platform Module) or SGX/TDX for remote attestation.
    - **Browser**: Leverage **WebAuthn** with platform-authenticator attestation or **Device Integrity APIs** (like Android Play Integrity / Apple App Attest).
**Action Plan**: Transition to a **Token-Exchange Handshake** where a new node must provide an attestation blob verified by a subset of "Validator" nodes.

## 2. State Consistency: The "CheatEngine" Vulnerability
**The Issue**: Raw SharedArrayBuffer (SAB) memory is the "Ground Truth." Any local modification (browser plugin, debugger) is immediately accepted by the kernel.
**Educational Context**: Because INOS uses a "Trust-on-Read" model for performance, it ignores the basic security principle: **Never trust the client memory.**
**Production-Grade Solution**:
- **Merkle-State-Transitions**: Treat the `CreditAccount` not as a raw variable, but as a leaf in a Merkle Tree. Every update must generate a new Root.
- **Signed Checkpoints**: Use Ed25519 (or PQ-safe alternatives below) to sign the state at the end of every epoch.
**Action Plan**: Implement **Sealed Credits**. Modules can increment "Pending Credits" in SAB, but only the `CreditSupervisor` can "Finalize" them into a signed state that the mesh will accept.

## 3. Cryptographic Guardrails: Post-Quantum & Side-Channel Resilience
**The Issue**: Execution in a shared WASM/JS environment creates timing and memory-access side-channels. Furthermore, Ed25519/AES are vulnerable to future quantum adversaries.
**Research & Inferences**:
- **[FAEST](https://faest.info/)**: A Digital Signature based on AES. It is uniquely suited for us because it uses **VOLE-in-the-head** ZK proofs. If an attacker uses FAEST, we can verify the signature without exposing the symmetric key to the same side-channel risks as classic asymmetric schemes.
- **[SPHINCS+](https://sphincs.org/)**: A stateless hash-based signature scheme. It is the gold standard for P2P security because it doesn't rely on number-theoretic assumptions (which quantum computers break).
**Production-Grade Solution**:
- **Isolated Signer Proxy**: Move private keys to a dedicated **Secure Worker** with NO access to the SharedArrayBuffer. Communication happens via a narrow, audited message-passing bridge.
- **Hybrid Cryptography**: Use Ed25519 for speed in non-critical tasks, but use **SPHINCS+** for Ledger Finalization and Node Identity.
**Action Plan**: Replace the embedded Crypto logic with a **Sanctuary Service**. The Kernel never sees the private key; it only receives signatures from the Sanctuary.

## 4. The "Copy Tax" & The Split Memory Challenge
**The Issue**: The "Zero-Copy" claim is violated by dependencies on `wasm-bindgen`, `js-sys`, and `web-sys`. Simultaneously, the Go WASM runtime forces a **Split Memory Architecture**.
**Educational Context**: 
- **Go Constraints**: Standard Go WASM cannot natively import a `SharedArrayBuffer` as its primary linear memory. This forces the use of a **Twin Memory Pattern** where the kernel maintains a local replica of the SAB state.
- **Copy Tax**: Every JS-bridge transition (especially via `web-sys`) triggers allocations.
**Production-Grade Solution**:
- **Synchronized Memory Twins**: Accept the Go-imposed copy as a **Consistency Boundary (Snapshot Isolation)**. By using `ReadAt` into ephemeral fixed buffers, we eliminate GC pressure while gaining immunity to "tearing reads" from high-frequency Rust modules.
- **HAL-Level Offloading**: Move the "Twin Sync" logic from the application code into the **INOS-HAL**.
**Action Plan**: Refactor the `SABBridge` into the HAL, ensuring all "Twin" copies use zero-allocation `ReadAt` patterns. Implement the **Linear Memory Mapper** for non-SAB environments to maintain a unified address space for modules.
**Production-Grade Solution**:
- **Native HAL (mmap)**: On native hosts, the Go Kernel creates a shared memory file (`/dev/shm/inos_sab`) and maps it. The Rust SDK uses the `memmap2` crate to point to the same address.
- **Go Kernel Refactor**: Split the kernel into `kernel-core` (platform-agnostic logic) and `kernel-host-wasm` / `kernel-host-native`.
**Action Plan**: Build the **INOS-HAL Trait**. 
```rust
pub trait MemoryProvider {
    fn read_atomic(&self, offset: usize) -> i32;
    fn write_atomic(&self, offset: usize, val: i32);
    // On systems without shared memory support (rare), 
    // this can fallback to synchronous RPC, tho with high latency.
}
```

## 5. Zombie Nodes & Resource Hijacking
**The Issue**: Threat actors can create "Headless" browser zombies that farm credits while providing zero value to the mesh.
**Production-Grade Solution**:
- **Proof-of-Active-Presence (PoAP)**: High-value credit rewards (Yield/UBI) should require occasional interactive proofs or signed telemetry from the UI thread (browser visibility API).
- **Reputation-Gated Scheduling**: Only delegate work to nodes that have a secondary verification (e.g., connected social accounts or a history of valid, signed PoUW results).
**Action Plan**: Update the `MeshScheduler` to favor "Attested" nodes for high-priority computation jobs.
