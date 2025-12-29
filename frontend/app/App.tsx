import { useEffect } from 'react';
import { useSystemStore } from '../src/store/system';
import StoryView from './components/StoryView';

export default function App() {
  const { status, error, initialize } = useSystemStore();
  const loading = status === 'booting' || status === 'initializing';
  const ready = status === 'ready';

  useEffect(() => {
    initialize();
  }, [initialize]);

  if (error) {
    return (
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          height: '100vh',
          flexDirection: 'column',
          gap: '1rem',
          background: 'linear-gradient(135deg, #0a0a0a 0%, #1a1a2e 100%)',
          color: '#ffffff',
        }}
      >
        <h1 style={{ color: '#ff4444' }}>Kernel Load Error</h1>
        <p>{error.message}</p>
        <button
          onClick={() => window.location.reload()}
          style={{
            padding: '0.75rem 1.5rem',
            background: 'rgba(255, 255, 255, 0.1)',
            border: '1px solid rgba(255, 255, 255, 0.2)',
            borderRadius: '8px',
            color: '#ffffff',
            cursor: 'pointer',
            fontSize: '1rem',
          }}
        >
          Reload
        </button>
      </div>
    );
  }

  if (loading || !ready) {
    return (
      <>
        <style>{`
          @keyframes spin {
            to { transform: rotate(360deg); }
          }
          @keyframes pulse {
            0%, 100% { opacity: 1; }
            50% { opacity: 0.5; }
          }
        `}</style>
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            height: '100vh',
            flexDirection: 'column',
            gap: '1.5rem',
            background: 'linear-gradient(135deg, #0a0a0a 0%, #1a1a2e 100%)',
            color: '#ffffff',
          }}
        >
          <div
            style={{
              width: '60px',
              height: '60px',
              border: '4px solid rgba(102, 126, 234, 0.2)',
              borderTop: '4px solid #667eea',
              borderRadius: '50%',
              animation: 'spin 1s linear infinite',
            }}
          />
          <div style={{ textAlign: 'center' }}>
            <p style={{ fontSize: '1.25rem', fontWeight: '600', marginBottom: '0.5rem' }}>
              Loading INOS Kernel
            </p>
            <p
              style={{
                fontSize: '0.875rem',
                color: 'rgba(255, 255, 255, 0.6)',
                animation: 'pulse 2s ease-in-out infinite',
              }}
            >
              Initializing distributed runtime...
            </p>
          </div>
        </div>
      </>
    );
  }

  return <StoryView />;
}
