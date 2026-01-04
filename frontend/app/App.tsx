import { useEffect } from 'react';
import { useSystemStore } from '../src/store/system';
import ArchitecturalBoids from './components/ArchitecturalBoids';
import ArchitecturalBlog from './components/ArchitecturalBlog';
import './styles/minimal.css';

export default function App() {
  const { status, error, initialize } = useSystemStore();
  const loading = status === 'booting' || status === 'initializing';
  const ready = status === 'ready';

  useEffect(() => {
    initialize();
  }, [initialize]);

  if (error) {
    return (
      <div className="minimal-app">
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            height: '100vh',
            flexDirection: 'column',
            gap: '2rem',
            padding: '2rem',
          }}
        >
          <div
            style={{
              maxWidth: '600px',
              width: '100%',
              padding: '3rem',
              background: 'var(--paper-white)',
              border: '2px solid var(--border-medium)',
              borderRadius: '2px',
              boxShadow: '0 4px 16px var(--shadow-medium)',
            }}
          >
            <h1
              style={{
                fontSize: 'var(--font-h1)',
                fontWeight: 700,
                marginBottom: '1rem',
                color: 'var(--ink-dark)',
                letterSpacing: '-0.02em',
              }}
            >
              Kernel Load Error
            </h1>
            <p
              style={{
                fontSize: 'var(--font-body)',
                color: 'var(--ink-medium)',
                marginBottom: '2rem',
                lineHeight: 1.6,
                fontFamily: 'var(--font-typewriter)',
                padding: '1rem',
                background: 'var(--paper-off-white)',
                border: '1px solid var(--border-subtle)',
                borderRadius: '2px',
              }}
            >
              {error.message}
            </p>
            <button
              onClick={() => window.location.reload()}
              className="minimal-button primary"
              style={{ width: '100%' }}
            >
              Reload System
            </button>
          </div>
        </div>
      </div>
    );
  }

  if (loading || !ready) {
    return (
      <div className="minimal-app">
        <style>{`
          @keyframes pulse {
            0%, 100% { transform: scale(1); opacity: 0.5; }
            50% { transform: scale(1.5); opacity: 1; }
          }
        `}</style>
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            height: '100vh',
            flexDirection: 'column',
          }}
        >
          {/* Minimal Loader */}
          <div
            style={{ position: 'relative', width: '40px', height: '40px', marginBottom: '2rem' }}
          >
            <div
              style={{
                position: 'absolute',
                top: 0,
                left: 0,
                width: '100%',
                height: '100%',
                border: '2px solid var(--sepia-accent)',
                animation: 'pulse 2s infinite ease-in-out',
              }}
            />
          </div>

          <h2 className="minimal-title" style={{ fontSize: '24px', margin: 0 }}>
            INOS Kernel
          </h2>
          <p className="minimal-text" style={{ fontSize: '14px', marginTop: '1rem' }}>
            Initializing distributed runtime...
          </p>
        </div>
      </div>
    );
  }

  // When ready, show the manifesto with the boids background
  return (
    <div className="minimal-app">
      <ArchitecturalBoids />
      <ArchitecturalBlog />
    </div>
  );
}
