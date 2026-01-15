@0xc42e032179831952;

using Base = import "/base/v1/base.capnp";

# INOS Unified Economic Ledger Schema v1.2
# Philosophy: "Performance-driven growth through incentivized exploration."

struct Wallet {
  metadata @19 :Base.Base.Metadata;
  publicKey @0 :Data;           # TSS Group Public Key
  balance @1 :Int64;             # Signed to allow debt for compute
  did @2 :Text;                  # root Identity (did:inos:...)
  
  # Incentive & Social Layer
  reputationScore @3 :Float32;
  linkedDevicesCount @4 :UInt16;   # UBI Multiplier catalyst
  uptimeScore @5 :Float32;
  lastUbiClaim @6 :Int64;
  
  # NEW: Social-Economic Graph
  creatorDid @7 :Text;           # Always "did:inos:nmxmxh"
  referrerDid @8 :Text;          # Default "did:inos:nmxmxh", changeable once
  referrerLockedAt @9 :Int64;    # Timestamp when referrer was locked (first PoUW)
  referrerChangedAt @10 :Int64;  # Timestamp of last referrer change (for cooldown)
  closeIdentities @11 :List(CloseIdentity);
  
  # Yield Tracking
  yieldEarned @12 :YieldStats;

  # Active Utility Metrics (Mirroring SAB state)
  earnedTotal @13 :UInt64;
  spentTotal @14 :UInt64;
  lastActivityEpoch @15 :UInt64;
  
  # Reputation/Tier levels
  tier @16 :EconomicTier;

  # Identity/Security Metadata
  threshold @17 :UInt8;          # 't' in (t, n) TSS
  totalShares @18 :UInt8;        # 'n' in (t, n) TSS
}

struct CloseIdentity {
  did @0 :Text;
  addedAt @1 :Int64;
  verifiedAt @2 :Int64;          # Mutual QR challenge timestamp
  relationship @3 :Text;         # Optional: "friend", "family", "colleague"
  yieldShare @4 :Float32;        # Auto-calculated: 0.5% / num_close_ids
  reputation @5 :Float32;        # Cached reputation at time of addition
}

struct YieldStats {
  # Earnings
  fromCreator @0 :UInt64;        # Credits earned as creator (nmxmxh only)
  fromReferrals @1 :UInt64;      # Credits earned from referred users
  fromCloseIds @2 :UInt64;       # Credits earned from close ID relationships
  
  # Payments
  paidToCreator @3 :UInt64;      # Credits paid to creator
  paidToReferrer @4 :UInt64;     # Credits paid to referrer
  paidToCloseIds @5 :UInt64;     # Credits paid to close IDs
  
  # Referral Stats
  referredUsers @6 :UInt32;      # Number of users referred
  activeReferrals @7 :UInt32;    # Referred users with ≥1 PoUW job
}

enum EconomicTier {
  basic @0;
  verified @1;
  contributor @2;
  validator @3;
  protocol @4; # Treasury/Mint nodes
}

struct Transaction {
  metadata @11 :Base.Base.Metadata;
  id @0 :Text;
  fromDid @1 :Text;              # Special: "Mint", "Treasury"
  toDid @2 :Text;
  amount @3 :UInt64;
  protocolFee @4 :UInt64;        # Fractional fee for protocol sustainability
  
  timestamp @5 :Int64;
  type @6 :TransactionType;
  
  # Context for PoUW and Incentives
  context :group {
    workId @7 :Text;             # For PoUW completion
    bountyId @8 :Text;           # For Growth Pool payouts
    patternId @9 :Text;          # For Developer Royalties
  }
  
  signature @10 :Data;
}

enum TransactionType {
  transfer @0;
  poUWCompletion @1;           # Worker reward minting
  protocolFeeCollection @2;    # Tax collection
  ubiDistribution @3;          # Social baseline
  bountyPayout @4;             # Incentive Layer rewards
  royaltyPayout @5;            # Creation-based rewards
  deviceLink @6;               # Governance/Incentive trigger
  creatorYield @7;             # NEW: Creator (nmxmxh) yield
  referrerYield @8;            # NEW: Referrer yield
  closeIdYield @9;             # NEW: Close ID yield
}

# Resource-specific metadata (merged from v1.1)
struct ContentPolicy {
  contentAddress @0 :Data;
  replicationTier @1 :ReplicationTier;
  replicaCount @2 :UInt32;
  accessCount @3 :UInt64;
  
  # Usage-based earning counters
  totalBytesServed @4 :UInt64;
  totalHoursStored @5 :UInt64;
}

enum ReplicationTier {
  hot @0;      # Performance/Caching
  warm @1;     # Balanced
  cold @2;     # Archive
}

struct EconomicSyscall {
  callerModuleId @0 :Text;
  maxCreditCost  @1 :Int64;      # Budgeting
  urgency        @2 :Int32;      # Scheduling Bias (-10 to +10)
}

