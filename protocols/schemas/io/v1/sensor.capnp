@0xb1c2d3e4f5a67890;

using Base = import "/base/v1/base.capnp";

interface IO {
  struct SensorFrame {
    sourceId @0 :Text;
    metadata @1: Base.Base.Metadata; # DNA attached to reading
    
    # Hard Real-Time Precision
    timestampNs @2 :Int64;      # UTC Nanoseconds (Absolute)
    monotonicNs @3 :Int64;      # System Monotonic Nanoseconds (Intervals)
    
    # Universal Input Handling
    union {
      rawBytes @4 :Data;            # Generic Binary (with mimetype in source metadata)
      
      # Low-Level Telemetry
      imu6Axis @5 :List(Float32);   # Accel + Gyro [ax,ay,az,gx,gy,gz]
      magnetometer @6 :List(Float32);
      gpsPosition @7 :GPS;
      
      # High-Bandwidth Perception
      depthMap @8 :Data;            # Compressed Depth Buffer (Draco/Zstd)
      lidarScan @9 :List(Point3D);  # Point Clouds
      holographicFrame @10 :Data;   # Volumetric Data / Lightfield
      
      # Audio / Video
      audioChunk @11 :Data;          # PCM / Opus
      videoFrame @12 :Data;          # H.264 / VP9 NAL Unit
      
      # Custom Expansion
      custom @13 :CustomData;
    }
  }
  
  struct GPS {
      latitude @0 :Float64;
      longitude @1 :Float64;
      altitude @2 :Float64;
      accuracy @3 :Float32;
  }
  
  struct CustomData {
     typeId @0 :Text;     # e.g. "my_robot:leg_stress"
     payload @1 :Data;
  }
  
  struct Point3D {
    x @0 :Float32;
    y @1 :Float32;
    z @2 :Float32;
    intensity @3 :Float32;
  }
  
  struct ControlCommand {
    targetId @0 :Text;
    action @1 :Text; # "move_to", "set_brightness"
    params @2 :List(Float32);
  }
}
