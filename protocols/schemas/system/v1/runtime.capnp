@0x9e8f7a6b5c4d3e2f;

interface Runtime {
  enum RuntimeRole {
    synapse @0; # High-throughput router (e.g., Firefox, Native Relay)
    neuron @1;  # Compute engine (e.g., Chromium, Node.js)
    sentry @2;  # Observer/Storage (e.g., Safari, Mobile)
  }

  struct RuntimeCapabilities {
    computeScore @0 :Float32;      # Normalized Compute Score (0.0 - 1.0+)
    networkLatency @1 :Float32;    # Loopback RTT in ms
    atomicsOverhead @2 :Float32;   # Atomics.wait overhead in ns
    
    # Detailed Capability Flags
    hasSimd @3 :Bool;
    hasGpu @4 :Bool;
    isHeadless @5 :Bool;
    batteryLevel @6 :Float32;      # 1.0 = Full, negative if unknown/plugged
  }
}
