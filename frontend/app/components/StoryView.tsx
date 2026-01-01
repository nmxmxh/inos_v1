import { useSystemStore } from '../../src/store/system';
import ParticleCanvas from './ParticleCanvas';
import PerformanceHUD from './PerformanceHUD';

export default function StoryView() {
  const { status, units } = useSystemStore();
  const activeUnits = Object.values(units).filter(u => u.active);

  // Module descriptions based on architecture
  const moduleStories: Record<string, { title: string; role: string; capabilities: string }> = {
    compute: {
      title: 'The Muscle',
      role: 'GPU-accelerated computation via WebGPU delegation',
      capabilities:
        'Particle systems, N-body physics, PBR rendering, procedural generation, post-processing effects',
    },
    science: {
      title: 'The Physicist',
      role: 'Deterministic physics simulation with conservation laws',
      capabilities:
        'Kinetic energy tracking, flux anchoring, mosaic registry, substance composition, conservation enforcement',
    },
    ml: {
      title: 'The Mind',
      role: 'Distributed AI inference with layer partitioning',
      capabilities:
        'Model loading, inference execution, PoR verification, distributed training, pattern recognition',
    },
    mining: {
      title: 'The Harvester',
      role: 'Background proof-of-work mining during idle cycles',
      capabilities:
        'SHA-256 mining, difficulty adjustment, pool integration, silent throttling, yield aggregation',
    },
    vault: {
      title: 'The Keeper',
      role: 'Content-addressed storage with economic incentives',
      capabilities:
        'CAS chunking, Brotli compression, replication management, cold/hot tier optimization',
    },
    drivers: {
      title: 'The Senses',
      role: 'Hardware I/O and sensor integration',
      capabilities:
        'Serial/USB/BLE communication, sensor data acquisition, device enumeration, real-time streaming',
    },
    diagnostics: {
      title: 'The Observer',
      role: 'System health monitoring and telemetry',
      capabilities:
        'Performance profiling, error tracking, resource monitoring, anomaly detection, metrics aggregation',
    },
  };

  return (
    <div
      style={{
        minHeight: '100vh',
        background: '#ffffff',
        color: '#000000',
        position: 'relative',
      }}
    >
      {/* Particle background */}
      <ParticleCanvas />

      {/* Main content */}
      <article
        style={{
          maxWidth: '900px',
          margin: '0 auto',
          padding: '120px 48px 120px',
          position: 'relative',
          zIndex: 1,
        }}
      >
        {/* Header */}
        <header style={{ marginBottom: '96px' }}>
          <div
            style={{
              fontSize: '11px',
              letterSpacing: '3px',
              fontWeight: 700,
              color: '#999',
              marginBottom: '16px',
            }}
          >
            INOS v2.0 • PRODUCTION-READY
          </div>

          <h1
            style={{
              fontSize: '64px',
              fontWeight: 200,
              lineHeight: 1.1,
              marginBottom: '32px',
              letterSpacing: '-3px',
            }}
          >
            The Internet-Native
            <br />
            Operating System
          </h1>

          <p
            style={{
              fontSize: '20px',
              lineHeight: 1.7,
              color: '#555',
              marginBottom: '32px',
              fontWeight: 300,
            }}
          >
            A biological runtime for the internet age—where nodes are cells, the kernel is the
            nervous system, and reactive mutation replaces message passing.
          </p>

          <div
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: '10px',
              padding: '8px 16px',
              background: status === 'ready' ? '#f0fdf4' : '#fef3c7',
              border: `1px solid ${status === 'ready' ? '#10b981' : '#f59e0b'}`,
              borderRadius: '8px',
              fontSize: '12px',
              fontWeight: 600,
              color: status === 'ready' ? '#10b981' : '#f59e0b',
              letterSpacing: '1px',
            }}
          >
            <div
              style={{
                width: '8px',
                height: '8px',
                borderRadius: '50%',
                background: status === 'ready' ? '#10b981' : '#f59e0b',
                boxShadow: `0 0 8px ${status === 'ready' ? '#10b981' : '#f59e0b'}`,
              }}
            />
            {status === 'ready' ? 'KERNEL ACTIVE' : 'INITIALIZING'}
          </div>
        </header>

        {/* Core Innovation */}
        <section style={{ marginBottom: '96px' }}>
          <h2
            style={{
              fontSize: '36px',
              fontWeight: 300,
              marginBottom: '32px',
              letterSpacing: '-1.5px',
            }}
          >
            The Core Innovation
          </h2>

          <div
            style={{
              padding: '32px',
              background: '#f8f9fa',
              borderLeft: '4px solid #3b82f6',
              marginBottom: '32px',
            }}
          >
            <h3 style={{ fontSize: '18px', fontWeight: 600, marginBottom: '16px' }}>
              Reactive Mutation
            </h3>
            <p style={{ fontSize: '16px', lineHeight: 1.7, color: '#555', marginBottom: '16px' }}>
              We replace traditional message passing with <strong>shared reality</strong>:
            </p>
            <div
              style={{
                fontFamily: 'monospace',
                fontSize: '13px',
                background: '#fff',
                padding: '16px',
                borderRadius: '4px',
                lineHeight: 1.8,
              }}
            >
              <div style={{ color: '#999' }}>// Traditional</div>
              <div>Node A → (serialize) → network → (deserialize) → Node B</div>
              <div style={{ marginTop: '12px', color: '#999' }}>// INOS</div>
              <div style={{ color: '#10b981' }}>
                Node A writes to SAB → Node B reads from same memory
              </div>
            </div>
            <p style={{ fontSize: '14px', color: '#666', marginTop: '16px', fontStyle: 'italic' }}>
              Result: Zero serialization overhead, atomic consistency, O(1) performance.
            </p>
          </div>
        </section>

        {/* The Three Layers */}
        <section style={{ marginBottom: '96px' }}>
          <h2
            style={{
              fontSize: '36px',
              fontWeight: 300,
              marginBottom: '48px',
              letterSpacing: '-1.5px',
            }}
          >
            Three Layers, One Memory
          </h2>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '48px' }}>
            {[
              {
                number: '01',
                title: 'Perception',
                tech: 'JavaScript',
                desc: 'The sensory layer captures hardware events and renders at 120Hz directly from SharedArrayBuffer. No serialization. No copies. Just raw, zero-latency perception.',
                color: '#3b82f6',
                examples: 'Mouse tracking, keyboard input, sensor data, DOM rendering',
              },
              {
                number: '02',
                title: 'Transformation',
                tech: 'Rust/WASM',
                desc: 'The computational layer where physics engines, ML inference, and scientific simulations run in parallel. Each module writes results directly to shared memory as a sovereign compute unit.',
                color: '#8b5cf6',
                examples: 'GPU compute, N-body physics, AI inference, image processing',
              },
              {
                number: '03',
                title: 'Coordination',
                tech: 'Go/WASM',
                desc: 'The orchestration layer observes completion flags via atomic epochs, gossips state deltas across the P2P mesh, and coordinates distributed consensus—all without blocking other layers.',
                color: '#ec4899',
                examples: 'Module lifecycle, mesh coordination, policy enforcement, syscalls',
              },
            ].map(layer => (
              <div
                key={layer.number}
                style={{ borderLeft: `4px solid ${layer.color}`, paddingLeft: '32px' }}
              >
                <div
                  style={{
                    fontSize: '12px',
                    fontWeight: 700,
                    color: layer.color,
                    letterSpacing: '2px',
                    marginBottom: '12px',
                  }}
                >
                  LAYER {layer.number}
                </div>
                <h3 style={{ fontSize: '28px', fontWeight: 600, marginBottom: '8px' }}>
                  {layer.title}
                </h3>
                <div
                  style={{
                    fontSize: '13px',
                    color: '#999',
                    marginBottom: '20px',
                    fontFamily: 'monospace',
                  }}
                >
                  {layer.tech}
                </div>
                <p
                  style={{ fontSize: '17px', lineHeight: 1.8, color: '#555', marginBottom: '16px' }}
                >
                  {layer.desc}
                </p>
                <div style={{ fontSize: '14px', color: '#888', fontStyle: 'italic' }}>
                  Examples: {layer.examples}
                </div>
              </div>
            ))}
          </div>
        </section>

        {/* Active Modules */}
        <section style={{ marginBottom: '96px' }}>
          <h2
            style={{
              fontSize: '36px',
              fontWeight: 300,
              marginBottom: '24px',
              letterSpacing: '-1.5px',
            }}
          >
            Active Compute Units
          </h2>

          <p style={{ fontSize: '17px', color: '#666', marginBottom: '48px', lineHeight: 1.7 }}>
            {activeUnits.length} of 7 specialized modules are online, each contributing unique
            capabilities to the distributed mesh. Every module is a sovereign compute unit with its
            own memory region and lifecycle.
          </p>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '32px' }}>
            {activeUnits.map(unit => {
              const story = moduleStories[unit.id] || {
                title: unit.id,
                role: 'Specialized compute unit',
                capabilities: 'Various capabilities',
              };

              return (
                <div
                  key={unit.id}
                  style={{
                    padding: '32px',
                    border: '1px solid #e5e5e5',
                    borderRadius: '12px',
                    transition: 'all 0.3s ease',
                    background: '#fff',
                  }}
                  onMouseEnter={e => {
                    e.currentTarget.style.borderColor = '#3b82f6';
                    e.currentTarget.style.boxShadow = '0 4px 12px rgba(59, 130, 246, 0.1)';
                    e.currentTarget.style.transform = 'translateY(-2px)';
                  }}
                  onMouseLeave={e => {
                    e.currentTarget.style.borderColor = '#e5e5e5';
                    e.currentTarget.style.boxShadow = 'none';
                    e.currentTarget.style.transform = 'translateY(0)';
                  }}
                >
                  <div
                    style={{
                      display: 'flex',
                      justifyContent: 'space-between',
                      marginBottom: '16px',
                    }}
                  >
                    <div>
                      <h3
                        style={{
                          fontSize: '22px',
                          fontWeight: 600,
                          marginBottom: '4px',
                          textTransform: 'uppercase',
                          letterSpacing: '0.5px',
                        }}
                      >
                        {unit.id}
                      </h3>
                      <div style={{ fontSize: '14px', color: '#3b82f6', fontWeight: 600 }}>
                        {story.title}
                      </div>
                    </div>
                    <div
                      style={{
                        width: '12px',
                        height: '12px',
                        borderRadius: '50%',
                        background: '#10b981',
                        boxShadow: '0 0 8px rgba(16, 185, 129, 0.5)',
                      }}
                    />
                  </div>

                  <p
                    style={{
                      fontSize: '15px',
                      color: '#666',
                      marginBottom: '20px',
                      lineHeight: 1.7,
                    }}
                  >
                    {story.role}
                  </p>

                  <div style={{ marginBottom: '20px' }}>
                    <div
                      style={{
                        fontSize: '12px',
                        color: '#999',
                        marginBottom: '8px',
                        fontWeight: 600,
                      }}
                    >
                      CAPABILITIES
                    </div>
                    <div style={{ fontSize: '14px', color: '#555', lineHeight: 1.6 }}>
                      {story.capabilities}
                    </div>
                  </div>

                  <div
                    style={{
                      display: 'flex',
                      gap: '24px',
                      fontSize: '12px',
                      color: '#999',
                      fontFamily: 'monospace',
                    }}
                  >
                    <div>
                      METHODS:{' '}
                      <span style={{ color: '#000', fontWeight: 600 }}>
                        {unit.capabilities?.length || 0}
                      </span>
                    </div>
                    <div>
                      VERSION: <span style={{ color: '#000', fontWeight: 600 }}>1.0.0</span>
                    </div>
                    <div>
                      MEMORY: <span style={{ color: '#000', fontWeight: 600 }}>16MB</span>
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        </section>

        {/* Architecture Principles */}
        <section style={{ marginBottom: '96px' }}>
          <h2
            style={{
              fontSize: '36px',
              fontWeight: 300,
              marginBottom: '48px',
              letterSpacing: '-1.5px',
            }}
          >
            Architectural Principles
          </h2>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '32px' }}>
            {[
              {
                principle: 'Zero-Copy Communication',
                desc: 'All inter-module communication happens through direct SharedArrayBuffer access. No serialization, no copying, no waiting.',
              },
              {
                principle: 'Epoch-Based Reactivity',
                desc: 'Modules signal completion by incrementing atomic counters. Observers react to changes without polling or callbacks.',
              },
              {
                principle: 'Sovereign Compute Units',
                desc: 'Each module is self-contained with its own memory region and lifecycle. No shared state except the coordination buffer.',
              },
              {
                principle: 'Concurrent Mutation',
                desc: 'Multiple layers mutate different SAB regions simultaneously, orchestrated by atomic operations and circuit breakers.',
              },
            ].map((item, i) => (
              <div key={i} style={{ padding: '24px', background: '#f8f9fa', borderRadius: '8px' }}>
                <h3 style={{ fontSize: '16px', fontWeight: 600, marginBottom: '12px' }}>
                  {item.principle}
                </h3>
                <p style={{ fontSize: '14px', lineHeight: 1.7, color: '#666' }}>{item.desc}</p>
              </div>
            ))}
          </div>
        </section>

        {/* Footer */}
        <footer
          style={{
            paddingTop: '64px',
            borderTop: '1px solid #e5e5e5',
            fontSize: '14px',
            color: '#999',
            textAlign: 'center',
          }}
        >
          <p style={{ marginBottom: '8px' }}>
            Built with JavaScript, Rust, and Go.
            <br />
            Powered by SharedArrayBuffer, WebAssembly, and Cap'n Proto.
          </p>
          <p style={{ fontSize: '12px', color: '#ccc' }}>
            Post-AI Development Paradigm • Intentional Architecture • Biological Runtime
          </p>
        </footer>
      </article>

      {/* Performance HUD */}
      <PerformanceHUD />
    </div>
  );
}
