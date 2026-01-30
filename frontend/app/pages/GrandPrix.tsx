import { useEffect, useState, useRef } from 'react';
import { Canvas } from '@react-three/fiber';
import { OrbitControls, PerspectiveCamera, Environment } from '@react-three/drei';
import { motion, AnimatePresence } from 'framer-motion';
import styled from 'styled-components';
import { dispatch } from '../../src/wasm/dispatch';
import { useSystemStore } from '../../src/store/system';
import DroneModel from '../features/grandprix/DroneModel';
import RaceTrack from '../features/grandprix/RaceTrack';
import { SAB_OFFSETS } from '../../src/racing/layout';

// Physics Component inside Canvas
// Physics is handled by autonomous worker via dispatch.plug
// No need for per-frame main thread overhead

const LoaderContainer = styled(motion.div)`
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: #000;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  z-index: 10;
  font-family: 'Inter', monospace;
`;

const LoaderText = styled(motion.div)`
  color: #10b981;
  font-size: 0.9rem;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  margin-top: 1rem;
`;

const ProgressBar = styled(motion.div)`
  width: 200px;
  height: 2px;
  background: #333;
  margin-top: 1rem;
  overflow: hidden;
`;

const ProgressFill = styled(motion.div)`
  height: 100%;
  background: #10b981;
`;

export default function GrandPrix() {
  const sab = useSystemStore(s => s.sab);
  const status = useSystemStore(s => s.status);
  const [ready, setReady] = useState(false);
  const [loadingStep, setLoadingStep] = useState('Waiting for Kernel...');
  const [error, setError] = useState<string | null>(null);
  const initializedRef = useRef(false);

  // Input State
  const inputRef = useRef({
    throttle: 0.0,
    pitch: 0.0,
    roll: 0.0,
    yaw: 0.0,
  });

  useEffect(() => {
    // Wait for basic system readiness
    if (!sab || status !== 'ready' || initializedRef.current) return;

    const init = async () => {
      initializedRef.current = true; // Mark as running only after we confirm readiness
      setLoadingStep('Initializing Drone Physics...');
      console.log('[GrandPrix] System ready. Starting initialization sequence...');

      try {
        // 1. Initialize State (Async to Worker, but guaranteed ready now)
        console.log('[GrandPrix] 1. Executing drone:init (Async/Worker)...');
        const t0 = performance.now();
        // FORCE SYNC REMOVED: We now rely on intelligent waiting for worker readiness
        await dispatch.execute('drone', 'init', { count: 8 });
        console.log(
          `[GrandPrix] 1. drone:init completed in ${(performance.now() - t0).toFixed(2)}ms`
        );

        // 2. Plug into Autonomous Worker Loop (Zero-CPU on main thread)
        console.log('[GrandPrix] 2. Plugging into worker loop...');
        const t1 = performance.now();
        await dispatch.plug('drone', 'simulation', {
          library: 'drone',
          method: 'step_physics',
          dt: 0.004,
          count: 8,
        });
        console.log(`[GrandPrix] 2. Plug completed in ${(performance.now() - t1).toFixed(2)}ms`);

        setLoadingStep('Ready');
        console.log('[GrandPrix] Initialization complete. Setting Ready.');
        setTimeout(() => setReady(true), 500);
      } catch (e) {
        console.error('[GrandPrix] Failed to init physics:', e);
        setError(e instanceof Error ? e.message : 'Unknown error');
        initializedRef.current = false; // Allow retry on error
      }
    };
    init();
  }, [sab, status]);

  // Keyboard Capture (unchanged)
  useEffect(() => {
    if (!sab) return;
    const controlView = new Float32Array(sab, SAB_OFFSETS.DRONE_CONTROL, 32);

    const handleKey = (e: KeyboardEvent, isDown: boolean) => {
      switch (e.key) {
        case 'w':
          inputRef.current.throttle = isDown ? 0.6 : 0.0;
          break;
        case 's':
          inputRef.current.throttle = isDown ? 0.0 : 0.0;
          break;
        case 'ArrowUp':
          inputRef.current.pitch = isDown ? -1.0 : 0.0;
          break;
        case 'ArrowDown':
          inputRef.current.pitch = isDown ? 1.0 : 0.0;
          break;
        case 'ArrowLeft':
          inputRef.current.roll = isDown ? -1.0 : 0.0;
          break;
        case 'ArrowRight':
          inputRef.current.roll = isDown ? 1.0 : 0.0;
          break;
        case 'a':
          inputRef.current.yaw = isDown ? -1.0 : 0.0;
          break;
        case 'd':
          inputRef.current.yaw = isDown ? 1.0 : 0.0;
          break;
      }
      controlView[0] = inputRef.current.throttle;
      controlView[1] = inputRef.current.pitch;
      controlView[2] = inputRef.current.roll;
      controlView[3] = inputRef.current.yaw;
    };

    const down = (e: KeyboardEvent) => handleKey(e, true);
    const up = (e: KeyboardEvent) => handleKey(e, false);
    window.addEventListener('keydown', down);
    window.addEventListener('keyup', up);
    return () => {
      window.removeEventListener('keydown', down);
      window.removeEventListener('keyup', up);
    };
  }, [sab]);

  return (
    <div style={{ width: '100vw', height: '100vh', background: '#111', position: 'relative' }}>
      <AnimatePresence>
        {!ready && (
          <LoaderContainer
            initial={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.5 }}
          >
            <LoaderText
              animate={{ opacity: [0.5, 1, 0.5] }}
              transition={{ repeat: Infinity, duration: 1.5 }}
            >
              {error ? `Error: ${error}` : loadingStep}
            </LoaderText>
            {!error && (
              <ProgressBar>
                <ProgressFill
                  initial={{ width: '0%' }}
                  animate={{ width: '100%' }}
                  transition={{ duration: 2, ease: 'easeInOut' }}
                />
              </ProgressBar>
            )}
            {error && (
              <div
                style={{ marginTop: 20, cursor: 'pointer', color: '#666' }}
                onClick={() => window.location.reload()}
              >
                Click to Retry
              </div>
            )}
          </LoaderContainer>
        )}
      </AnimatePresence>

      <Canvas shadows dpr={[1, 2]}>
        <PerspectiveCamera makeDefault position={[0, 2, 5]} fov={75} />
        <OrbitControls makeDefault />
        <ambientLight intensity={0.5} />
        <spotLight position={[10, 10, 10]} angle={0.15} penumbra={1} intensity={1} castShadow />
        <Environment preset="city" />

        {ready && (
          <>
            <RaceTrack />
            <DroneModel />
          </>
        )}
      </Canvas>

      {/* HUD Overlay */}
      {ready && (
        <div
          style={{
            position: 'absolute',
            top: 20,
            left: 20,
            color: '#0f0',
            fontFamily: 'monospace',
            pointerEvents: 'none',
          }}
        >
          <h3>AI GRAND PRIX</h3>
          <p>Drone 0 Control:</p>
          <pre>{JSON.stringify(inputRef.current, null, 2)}</pre>
          <p>Controls: WASD + Arrows</p>
        </div>
      )}
    </div>
  );
}
