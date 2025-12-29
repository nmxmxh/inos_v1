@0xe5f6789012345678;

# Internal Kernel Protocol for managing the Node's Lifecycle

# Sent by Supervisor to Worker (or remote Node)
struct LifecycleCmd {
  id @0 :Text;
  type @1 :CommandType;
  targetCapsuleId @2 :Text;
  
  # Parameters for the command
  params @3 :Data; 
  
  timestamp @4 :Int64;
}

enum CommandType {
  spawn @0;       # Start a new Capsule instance
  kill @1;        # Forcefully terminate
  pause @2;       # Freeze execution (serialize state)
  resume @3;      # Thaw execution
  snapshot @4;    # Dump current memory to disk (CRIU logic)
  resizeMem @5;   # Grow/Shrink SharedArrayBuffer allocation
}

# Telemetry from the Running Capsule
struct HealthHeartbeat {
  capsuleId @0 :Text;
  status @1 :Status;
  
  memoryUsage @2 :UInt64;
  cpuTimeNs @3 :Int64;
  
  # Pressure Signals
  isCongested @4 :Bool; # "I can't keep up with input"
  needsMoreMem @5 :Bool;
}

enum Status {
  starting @0;
  healthy @1;
  degraded @2; # Running but slow
  zombie @3;   # Unresponsive
}

# Internal Kernel Protocol for managing the Node's Lifecycle
interface Orchestration {
  # Methods can be added here if needed for RPC
}

