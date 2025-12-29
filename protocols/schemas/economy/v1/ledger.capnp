@0xc42e032179831952;

interface Economy {
  struct Wallet {
    publicKey @0 :Data;
    balance @1 :UInt64;
  }

  struct Transaction {
    id @0 :Text;
    from @1 :Text; # Wallet ID (Public Key Hash)
    to @2 :Text;   # Wallet ID
    amount @3 :UInt64;
    signature @4 :Data;
    timestamp @5 :Int64;
  }
  
  struct MiningShare {
    nonce @0 :UInt64;
    hash @1 :Data;
    jobId @2 :Text;
  }
}
