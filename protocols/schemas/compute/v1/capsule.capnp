@0xa2c3d4e5f6789012;

using Base = import "/base/v1/base.capnp";

# Compute Capsule: Universal Job Execution Interface
# This schema defines the contract between the Go Kernel and Rust Compute module

interface Compute {
  
  # =================================================================
  # Job Specification (Kernel → Compute)
  # =================================================================
  
  struct JobRequest {
    jobId @0 :Text;              # Unique job identifier
    library @1 :Text;            # Library name: "image", "video", "audio", "crypto", "data", "gpu", "storage", "ml"
    method @2 :Text;             # Method name: "resize", "encode", "sha256", "store_chunk", etc.
    input @3 :Data;              # Input data (zero-copy reference to SAB)
    params @4 :Data;             # JSON (UTF-8 bytes) OR Binary Cap'n Proto (Tier 2)
    budget @5 :UInt64;           # Credits allocated for this job
    priority @6 :UInt8;          # 0-255 (higher = more urgent)
    timeout @7 :UInt64;          # Maximum execution time (ms)
    metadata @8 :Base.Base.Metadata;  # Standard metadata (user, device, trace)
  }
  
  # =================================================================\n  # Supported Libraries (Documentation)\n  # =================================================================\n  # \n  # image:   Image processing (resize, crop, filter, encode/decode)\n  # video:   Video transcoding (H.264, H.265, VP9, AV1)\n  # audio:   Audio processing (encode, decode, effects, FFT)\n  # crypto:  Cryptographic operations (hash, sign, verify, encrypt)\n  # data:    Data processing (Parquet, Arrow, Polars)\n  # gpu:     Custom GPU shaders (WGSL)\n  # ml:      ML inference (quantized LLMs) - future\n  # physics: Molecular dynamics - future\n  # \n  # Each library exposes its full API via method dispatch.\n  # Params are JSON-encoded for maximum flexibility.\n  # =================================================================
  
  # =================================================================
  # Job Parameters (Union)
  # =================================================================
  
  struct JobParams {
    union {
      imageParams @0 :ImageParams;
      videoParams @1 :VideoParams;
      cryptoParams @2 :CryptoParams;
      customParams @3 :CustomParams;
      mlParams @4 :MLParams;
      physicsParams @5 :PhysicsParams;
      audioParams @6 :AudioParams;
      render3dParams @7 :Render3DParams;
      dataParams @8 :DataParams;
    }
  }
  
  # --- Image Processing ---
  struct ImageParams {
    operation @0 :ImageOp;
    width @1 :UInt32;
    height @2 :UInt32;
    quality @3 :UInt8;          # 0-100 for JPEG/WebP
    format @4 :ImageFormat;
  }
  
  enum ImageOp {
    resize @0;
    crop @1;
    filter @2;
    encode @3;
    decode @4;
  }
  
  enum ImageFormat {
    jpeg @0;
    png @1;
    webp @2;
    avif @3;
  }
  
  # --- Video Encoding ---
  struct VideoParams {
    codec @0 :VideoCodec;
    bitrate @1 :UInt32;         # Target bitrate (kbps)
    fps @2 :UInt8;              # Frames per second
    width @3 :UInt32;
    height @4 :UInt32;
    keyframeInterval @5 :UInt16; # GOP size
  }
  
  enum VideoCodec {
    h264 @0;
    h265 @1;
    vp9 @2;
    av1 @3;
  }
  
  # --- Cryptographic Operations ---
  struct CryptoParams {
    operation @0 :CryptoOp;
    algorithm @1 :Text;         # e.g., "SHA256", "Ed25519", "AES-256-GCM"
    key @2 :Data;               # Optional key for sign/encrypt
  }
  
  enum CryptoOp {
    hash @0;
    sign @1;
    verify @2;
    encrypt @3;
    decrypt @4;
  }
  
  # --- Custom WGSL Shader ---
  struct CustomParams {
    shaderSource @0 :Text;      # WGSL shader code
    workgroupSize @1 :List(UInt32); # [x, y, z]
    bufferSizes @2 :List(UInt32);   # Sizes of input/output buffers
  }
  
  # --- ML Inference (Future) ---
  struct MLParams {
    modelId @0 :Text;           # Which model to use
    inputShape @1 :List(UInt32);
    outputShape @2 :List(UInt32);
  }
  
  # --- Physics Simulation (Future) ---
  struct PhysicsParams {
    deltaTime @0 :Float32;
    gravity @1 :List(Float32);  # [x, y, z]
    particleCount @2 :UInt32;
  }
  
  # --- Audio Processing ---
  struct AudioParams {
    operation @0 :AudioOp;
    sampleRate @1 :UInt32;      # Hz (e.g., 44100, 48000)
    channels @2 :UInt8;          # 1=mono, 2=stereo
    bitDepth @3 :UInt8;          # 16, 24, 32
    codec @4 :AudioCodec;
    effectParams @5 :Data;       # JSON or binary effect parameters
  }
  
  enum AudioOp {
    encode @0;
    decode @1;
    applyEffect @2;              # Reverb, EQ, compression, etc.
    analyze @3;                  # FFT, spectrum analysis
    normalize @4;
  }
  
  enum AudioCodec {
    mp3 @0;
    aac @1;
    opus @2;
    flac @3;
    wav @4;
  }
  
  # --- 3D Rendering ---
  struct Render3DParams {
    operation @0 :Render3DOp;
    width @1 :UInt32;
    height @2 :UInt32;
    samples @3 :UInt16;          # Anti-aliasing samples
    maxBounces @4 :UInt8;        # Ray tracing bounces
    meshData @5 :Data;           # glTF, OBJ, or custom format
    cameraParams @6 :CameraParams;
  }
  
  enum Render3DOp {
    rayTrace @0;
    rasterize @1;
    meshProcess @2;              # Simplification, UV unwrap
    bake @3;                     # Lightmap, AO baking
  }
  
  struct CameraParams {
    position @0 :List(Float32);  # [x, y, z]
    target @1 :List(Float32);    # [x, y, z]
    fov @2 :Float32;             # Field of view (degrees)
  }
  
  # --- Data Analysis ---
  struct DataParams {
    operation @0 :DataOp;
    inputFormat @1 :DataFormat;
    outputFormat @2 :DataFormat;
    analysisType @3 :AnalysisType;
    parameters @4 :Data;         # JSON parameters for specific analysis
  }
  
  enum DataOp {
    transform @0;                # Convert between formats
    aggregate @1;                # Sum, average, group by
    filter @2;                   # SQL-like filtering
    analyze @3;                  # Statistical analysis
    preprocess @4;               # ML preprocessing (normalize, encode)
  }
  
  enum DataFormat {
    json @0;
    csv @1;
    parquet @2;
    arrow @3;
    binary @4;
  }
  
  enum AnalysisType {
    statistics @0;               # Mean, median, std dev
    correlation @1;
    regression @2;
    clustering @3;
    timeSeries @4;
  }
  
  # =================================================================
  # Job Result (Compute → Kernel)
  # =================================================================
  
  struct JobResult {
    jobId @0 :Text;
    status @1 :Status;
    output @2 :Data;            # Result data (zero-copy reference to SAB)
    cost @3 :UInt64;            # Actual credits consumed
    executionTimeNs @4 :UInt64; # Execution time (nanoseconds)
    error @5 :Base.Base.Error;  # Error details if failed
    metrics @6 :ExecutionMetrics;
    errorMessage @7 :Text;      # Human-readable error (even on success for warnings)
    retryable @8 :Bool;         # Can this job be retried?
  }
  
  enum Status {
    success @0;
    failed @1;
    budgetExceeded @2;
    timeout @3;
    invalidParams @4;
  }
  
  # =================================================================
  # Execution Metrics
  # =================================================================
  
  struct ExecutionMetrics {
    cpuTimeNs @0 :UInt64;       # CPU time used
    gpuTimeNs @1 :UInt64;       # GPU time used (if applicable)
    memoryPeakBytes @2 :UInt64; # Peak memory usage
    inputBytes @3 :UInt64;      # Input data size
    outputBytes @4 :UInt64;     # Output data size
  }
}
