fn main() {
    ::capnpc::CompilerCommand::new()
        .file("../../protocols/schemas/ml/v1/model.capnp")
        .src_prefix("../../protocols/schemas")
        .import_path("../../protocols/schemas")
        .run()
        .expect("compiling schema");
}
