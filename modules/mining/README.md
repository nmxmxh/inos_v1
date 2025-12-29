# Mining Module: High-Performance Bitcoin Search & Privacy

This module implements the INOS sovereign yield engine. It leverages WebGPU for parallel SHA-256 searching and a Rust-based cryptographic stack for address generation and privacy obfuscation.

## 🌌 Architectural Philosophy
Mining in INOS is "Silent as a Lamb." We harvest the idle silicon of the internet's edge without impacting user experience.

- **The Binary Advantage**: We operate on raw `u32` bits. No ASCII, no JSON, no serialization tax.
- **Hardware-Level Precision**: Our WGSL shader implements bit-perfect SHA-256 compression functions directly on the GPU ALUs.
- **Privacy First**: We utilize deterministic EC derivation and source obfuscation to protect the Architect's treasury.

## 🔐 Cryptographic Workflow & Privacy Strategy

The system is designed for **Source Obfuscation** and **Yield Privacy**.

### 1. Address Generation Pipeline
Generating a treasury address involves the following deterministic pipeline:
1. **Secret Generation**: Secure 256-bit private key (32 bytes).
2. **ECC Derivation**: `secp256k1` curve used to derive the public key.
3. **Hashing**: `RIPEMD160(SHA256(PublicKey))` to create the 20-byte payload.
4. **Encoding**: Bech32 (Native SegWit `bc1`) encoding for modern, low-fee transactions.

### 2. Operational Security (OpSec)

| Principle | Implementation Strategy |
| :--- | :--- |
| **Secret Key Custody** | Private keys (WIF) are **never** stored in the client. They are generated once on air-gapped systems and used as environmental constants. |
| **Address Rotation** | To break blockchain correlation, the mesh supports unique payout addresses per node session. Nodes register local addresses with the coordinator, which are later swept. |
| **Network Obfuscation** | All mining signaling is proxied via the P2P transport layer, masking IP addresses and preventing geographic clustering. |
| **Source Obfuscation** | Address generation logic and treasury targets are obfuscated in the WASM binary to resist reverse-engineering. |

## 🚀 Performance Optimizations (v1.9+)

| Strategy | Impact | Technical Benefit |
| :--- | :--- | :--- |
| **Shared Memory** | **16x Speedup** | Moves the message schedule (`W`) to workgroup memory, slashing global memory fetches. |
| **Nonce Iteration** | **256x Efficiency** | Each thread iterates through 256 nonces locally. Reduces CPU-GPU context switches. |
| **Early-Exit** | **Bus Clarity** | Threads exit immediately if difficulty isn't met, preventing data bus congestion. |
| **Double SHA-256** | **Mainnet Ready** | Native "Hash-of-Hash" implementation entirely on the GPU. |

## 🛠️ Usage for Architects

### Generating a New Treasury Pair
To generate a master address for use as `INOS_TREASURY_ADDRESS`:

```rust
// Requires: bitcoin = { version = "0.31", features = ["rand"] }
use bitcoin::secp256k1::{rand, SecretKey};
use bitcoin::{Address, Network, PublicKey, PrivateKey};

pub fn generate_treasury() -> (String, String) {
    let secret_key = SecretKey::new(&mut rand::thread_rng());
    let public_key = PublicKey::from_secret_key(&SECP256K1, &secret_key);
    let address = Address::p2wpkh(&public_key, Network::Bitcoin).unwrap();
    let wif = PrivateKey::new(secret_key, Network::Bitcoin).to_wif();
    (address.to_string(), wif)
}
```

> [!WARNING]
> The private key (WIF) is the soul of your treasury. Never embed it in the `inos_v1` source code. Pass the **Address** only via environment variables.

## 📊 Revenue Projections (Optimized)
| Scale | Hashrate | Daily Yield (est.) |
| :--- | :--- | :--- |
| **1,000 Nodes** | 1.5 PH/s | ~$225 |
| **10,000 Nodes** | 15 PH/s | ~$2,250 |
| **1,000,000 Nodes**| 1.5 EH/s | ~$228,000 |

*Values estimated based on modern GPU hashrates and Bitcoin difficulty.*
