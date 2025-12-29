extern crate capnpc;

fn main() {
    // Re-run if schemas change
    println!("cargo:rerun-if-changed=../../protocols/schemas");

    ::capnpc::CompilerCommand::new()
        .file("../../protocols/schemas/base/v1/base.capnp")
        .file("../../protocols/schemas/compute/v1/capsule.capnp")
        .file("../../protocols/schemas/system/v1/orchestration.capnp")
        .file("../../protocols/schemas/io/v1/actor.capnp")
        .file("../../protocols/schemas/io/v1/sensor.capnp")
        .file("../../protocols/schemas/economy/v1/ledger.capnp")
        .file("../../protocols/schemas/system/v1/syscall.capnp")
        .src_prefix("../../protocols/schemas")
        // Add import path so they can find each other
        .import_path("../../protocols/schemas")
        .run()
        .expect("schema compiler command");
}
