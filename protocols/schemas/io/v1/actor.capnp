@0xc3d4e5f678901234;

using Base = import "../../base/v1/base.capnp";

interface Actor {
  struct Command {
    id @0 :Text;
    targetId @1 :Text; # The definition of the Actor Node
    timestampNs @2 :Int64;
    
    metadata @3 :Base.Base.Metadata;
    
    union {
      # Generic
      rawBytes @4 :Data;
      
      # Robotics / Motion
      moveTo @5 :Pose3D;
      velocity @6 :Vector3;
      torque @7 :List(Float32); # Joint torques
      
      # Visual / Holographic
      displayFrame @8 :Data;
      setHologram @9 :HologramPatch;
      
      # Environment
      gpioSet @10 :List(GPIOState);
    }
  }
  
  struct Pose3D {
    position @0 :Vector3;
    rotation @1 :Quaternion;
  }
  
  struct Vector3 {
    x @0 :Float32;
    y @1 :Float32;
    z @2 :Float32;
  }
  
  struct Quaternion {
    x @0 :Float32;
    y @1 :Float32;
    z @2 :Float32;
    w @3 :Float32;
  }
  
  struct GPIOState {
    pin @0 :UInt16;
    value @1 :Bool;
  }
  
  struct HologramPatch {
    assetId @0 :Text;
    transform @1 :Pose3D;
    scale @2 :Vector3;
    state @3 :Data; # Shader uniforms or state
  }
}
