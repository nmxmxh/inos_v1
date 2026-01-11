/**
 * INOS Technical Codex — Main Application
 *
 * Integrates React Router with system initialization.
 */

import { useEffect } from 'react';
import { BrowserRouter, Routes, Route, useLocation } from 'react-router-dom';
import { useSystemStore } from '../src/store/system';
import './styles/minimal.css';

// Scroll to top on route change
function ScrollToTop() {
  const { pathname } = useLocation();
  useEffect(() => {
    window.scrollTo(0, 0);
  }, [pathname]);
  return null;
}

// Layout
import Layout from './ui/Layout';

// Pages
import Landing from './pages/Landing';
import Problem from './pages/Problem';
import Insight from './pages/Insight';
import Architecture from './pages/Architecture';
import Genesis from './pages/Genesis';
import Cosmos from './pages/Cosmos';

// Deep Dives
import { ZeroCopy, Signaling, Mesh, Economy, Threads, Graphics, Database } from './pages/DeepDives';
import Diagnostics from './pages/Diagnostics';

function SystemLoader({ children }: { children: React.ReactNode }) {
  const status = useSystemStore(s => s.status);
  const error = useSystemStore(s => s.error);
  const initialize = useSystemStore(s => s.initialize);

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

  // System ready - render children
  return <>{children}</>;
}

export default function App() {
  return (
    <BrowserRouter>
      <ScrollToTop />
      <SystemLoader>
        <Routes>
          <Route path="/" element={<Layout />}>
            <Route index element={<Landing />} />
            <Route path="problem" element={<Problem />} />
            <Route path="insight" element={<Insight />} />
            <Route path="architecture" element={<Architecture />} />
            <Route path="genesis" element={<Genesis />} />
            <Route path="cosmos" element={<Cosmos />} />
            <Route path="diagnostics" element={<Diagnostics />} />
            {/* Deep Dives */}
            <Route path="deep-dives">
              <Route path="zero-copy" element={<ZeroCopy />} />
              <Route path="signaling" element={<Signaling />} />
              <Route path="mesh" element={<Mesh />} />
              <Route path="economy" element={<Economy />} />
              <Route path="threads" element={<Threads />} />
              <Route path="graphics" element={<Graphics />} />
              <Route path="database" element={<Database />} />
            </Route>
          </Route>
        </Routes>
      </SystemLoader>
    </BrowserRouter>
  );
}
