import { useSystemStore } from '../../src/store/system';
import PerformanceHUD from './PerformanceHUD';
import BirdCanvas from './BirdCanvas';

export default function StoryView() {
  const { status, units } = useSystemStore();

  return (
    <div className="minimal-app">
      {/* Sticky Header */}
      <header className="minimal-header">
        <div className="minimal-nav" style={{ justifyContent: 'space-between', width: '100%' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <div className="minimal-status active">INOS v1.9</div>
            <span style={{ fontWeight: 600, letterSpacing: '-0.5px' }}>
              Distributed Compute Mesh
            </span>
          </div>
          <div className="minimal-status loading">
            {status === 'ready' ? 'KERNEL ACTIVE' : 'BOOTING...'}
          </div>
        </div>
      </header>

      <main className="minimal-main">
        {/* Hero Section: The Narrative */}
        <section className="minimal-section">
          <h2 className="minimal-title" style={{ fontSize: '32px', marginBottom: '24px' }}>
            The Zero-Copy Constellation
          </h2>
          <div className="minimal-grid" style={{ gridTemplateColumns: '1fr 1fr' }}>
            <div>
              <p className="minimal-text" style={{ fontSize: '16px' }}>
                Welcome to the Internet-Native Operating System. Unlike traditional cloud
                architectures that rely on heavy serialization and network overhead, INOS runs
                entirely on bare-metal WebAssembly with <strong>zero-copy shared memory</strong>.
              </p>
              <p className="minimal-text" style={{ marginTop: '16px' }}>
                The units below are not microservices. They are autonomous, sandboxed WASM closures
                that read directly from the Kernel's memory without copying data. This allows for
                near-native performance (~10 TFLOPS latent) directly in the browser.
              </p>
            </div>

            {/* 3D Simulation Layer (GPU + Science + ML) */}
            <div style={{ minHeight: '400px', gridColumn: 'span 1' }}>
              <BirdCanvas />
            </div>
          </div>
        </section>

        {/* Live Modules Section */}
        <section>
          <div
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              marginBottom: '24px',
            }}
          >
            <h3 className="minimal-section-title-sm">Active Units</h3>
            <div className="minimal-status active">LIVE REGISTRY</div>
          </div>

          <div className="minimal-grid">
            {units &&
              Object.values(units).map(unit => (
                <div key={unit.id} className={`minimal-card ${unit.active ? 'active' : ''}`}>
                  <div className="minimal-card-header">
                    <div
                      style={{
                        display: 'flex',
                        justifyContent: 'space-between',
                        alignItems: 'center',
                      }}
                    >
                      <h3 className="minimal-card-title" style={{ textTransform: 'capitalize' }}>
                        {unit.id}
                      </h3>
                      <div className={`minimal-status ${unit.active ? 'active' : 'inactive'}`}>
                        {unit.active ? 'Active' : 'Offline'}
                      </div>
                    </div>
                  </div>

                  <div className="minimal-card-description">
                    {/* Capabilities List */}
                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px' }}>
                      {unit.capabilities && unit.capabilities.length > 0 ? (
                        unit.capabilities.map((cap, i) => (
                          <span key={i} className="minimal-badge">
                            {cap}
                          </span>
                        ))
                      ) : (
                        <span style={{ fontStyle: 'italic', opacity: 0.5 }}>Initializing...</span>
                      )}
                    </div>
                  </div>

                  <div className="minimal-card-actions">
                    <div style={{ fontSize: '11px', color: '#666' }}>
                      MEM: <span style={{ color: '#fff' }}>12MB</span>
                    </div>
                    <div style={{ fontSize: '11px', color: '#666' }}>
                      VER: <span style={{ color: '#fff' }}>1.0.0</span>
                    </div>
                  </div>
                </div>
              ))}
          </div>
        </section>

        {/* Spacer for fixed footer */}
        <div style={{ height: '80px' }} />
      </main>

      {/* Fixed Footer HUD */}
      <footer
        style={{
          position: 'fixed',
          bottom: 0,
          left: 0,
          right: 0,
          background: 'rgba(5,5,5,0.9)',
          borderTop: '1px solid var(--border-subtle)',
          backdropFilter: 'blur(20px)',
          zIndex: 100,
        }}
      >
        <PerformanceHUD />
      </footer>
    </div>
  );
}
