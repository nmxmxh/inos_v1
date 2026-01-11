// Cap'n Proto build script for Rust modules
// This automatically generates Rust code from .capnp schemas during build

fn main() {
    println!("cargo:rerun-if-changed=../protocols/schemas");

    // Configure Cap'n Proto compiler
    capnpc::CompilerCommand::new()
        .src_prefix("../protocols/schemas")
        // Base protocols
        .file("../protocols/schemas/base/v1/base.capnp")
        // System protocols
        .file("../protocols/schemas/system/v1/orchestration.capnp")
        .file("../protocols/schemas/system/v1/syscall.capnp")
        .file("../protocols/schemas/system/v1/sab_layout.capnp")
        .file("../protocols/schemas/system/v1/resource.capnp")
        // Compute protocols
        .file("../protocols/schemas/compute/v1/capsule.capnp")
        // I/O protocols
        .file("../protocols/schemas/io/v1/sensor.capnp")
        .file("../protocols/schemas/io/v1/actor.capnp")
        // Economy protocols
        .file("../protocols/schemas/economy/v1/ledger.capnp")
        // Identity protocols
        .file("../protocols/schemas/identity/v1/identity.capnp")
        .run()
        .expect("Cap'n Proto schema compilation failed");

    println!("cargo:warning=Cap'n Proto schemas compiled successfully");
}
