/**
 * INOS Technical Codex — Main Application
 *
 * Integrates React Router with system initialization.
 */

import { useEffect } from 'react';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { AnimatePresence, motion } from 'framer-motion';
import { Analytics } from '@vercel/analytics/react';
import { useSystemStore } from '../src/store/system';
import './styles/minimal.css';

// Layout
import Layout from './ui/Layout';
import MysticLoader from './ui/MysticLoader';

// Pages
import Landing from './pages/Landing';
import Problem from './pages/Problem';
import Insight from './pages/Insight';
import Architecture from './pages/Architecture';
import History from './pages/History';
import WhatsNext from './pages/WhatsNext';
import GrandPrix from './pages/GrandPrix';

// Deep Dives
import {
  ZeroCopy,
  Signaling,
  Mesh,
  Economy,
  Threads,
  Graphics,
  Database,
  Atomics,
  Performance,
  CapnProto,
} from './pages/DeepDives';
import Diagnostics from './pages/Diagnostics';
import ArchitecturalBoids from './features/boids/ArchitecturalBoids';

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

  return (
    <>
      <ArchitecturalBoids />
      <AnimatePresence mode="wait">
        {loading || !ready ? (
          <motion.div
            key="loader"
            exit={{ opacity: 0, transition: { duration: 0.8, ease: 'easeInOut' } }}
            style={{ position: 'fixed', inset: 0, zIndex: 9999 }}
          >
            <MysticLoader status={status} />
          </motion.div>
        ) : (
          <motion.div
            key="app"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            transition={{ duration: 1.2, ease: 'easeOut', delay: 0.2 }}
          >
            {children}
          </motion.div>
        )}
      </AnimatePresence>
    </>
  );
}

export default function App() {
  return (
    <>
      <Analytics />
      <BrowserRouter>
        <SystemLoader>
          <Routes>
            <Route path="/" element={<Layout />}>
              <Route index element={<Landing />} />
              <Route path="problem" element={<Problem />} />
              <Route path="insight" element={<Insight />} />
              <Route path="architecture" element={<Architecture />} />
              <Route path="history" element={<History />} />
              <Route path="whats-next" element={<WhatsNext />} />
              <Route path="diagnostics" element={<Diagnostics />} />
              <Route path="grandprix" element={<GrandPrix />} />
              {/* Deep Dives */}
              <Route path="deep-dives">
                <Route path="performance" element={<Performance />} />
                <Route path="zero-copy" element={<ZeroCopy />} />
                <Route path="capn-proto" element={<CapnProto />} />
                <Route path="signaling" element={<Signaling />} />
                <Route path="atomics" element={<Atomics />} />
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
    </>
  );
}
