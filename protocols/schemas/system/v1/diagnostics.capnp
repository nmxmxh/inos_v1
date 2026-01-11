@0xd3a2b1c4e5f67890;

struct DiagnosticsRequest {
  id @0 :UInt64;
  method @1 :Method;
  
  enum Method {
    ping @0;
    scanMemory @1;
    collectBridgeMetrics @2;
  }
}

struct DiagnosticsResponse {
  id @0 :UInt64;
  status @1 :Status;
  
  union {
    ok @2 :Void;
    metrics @3 :Data;
    error @4 :Text;
  }

  enum Status {
    success @0;
    error @1;
    unsupported @2;
  }
}
