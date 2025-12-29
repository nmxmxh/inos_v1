extern crate capnpc;

fn main() {
    // Re-run if schemas change
    println!("cargo:rerun-if-changed=../../protocols/schemas");

    ::capnpc::CompilerCommand::new()
        .file("../../protocols/schemas/science/v1/science.capnp")
        .src_prefix("../../protocols/schemas")
        // Add import path so it can find base.capnp
        .import_path("../../protocols/schemas")
        .run()
        .expect("schema compiler command");
}
