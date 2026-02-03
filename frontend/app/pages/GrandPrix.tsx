/**
 * AI Grand Prix - Autonomous Drone Racing
 *
 * Features:
 * - Chase camera following lead drone
 * - Light theme with grass track
 * - Real-time telemetry HUD
 * - Premium typography (Inter font)
 */

import { useEffect, useState, useRef } from 'react';
import { Canvas, useFrame, useThree } from '@react-three/fiber';
import { PerspectiveCamera } from '@react-three/drei';
import { EffectComposer, Bloom } from '@react-three/postprocessing';
import { motion, AnimatePresence } from 'framer-motion';
import styled from 'styled-components';
import * as THREE from 'three';
import { dispatch } from '../../src/wasm/dispatch';
import { useSystemStore } from '../../src/store/system';
import DroneModel from '../features/grandprix/DroneModel';
import RaceTrack from '../features/grandprix/RaceTrack';
import { SAB_OFFSETS, DRONE_CONSTANTS } from '../../src/racing/layout';

// ============= STYLED COMPONENTS (Light Theme) =============

const LoaderContainer = styled(motion.div)`
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: linear-gradient(135deg, #f0f9ff 0%, #e0f2fe 100%);
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  z-index: 10;
  font-family:
    'Inter',
    -apple-system,
    system-ui,
    sans-serif;
`;

const LoaderLogo = styled.div`
  font-size: 2.5rem;
  font-weight: 800;
  letter-spacing: -0.04em;
  color: #1e293b;
  margin-bottom: 0.5rem;
`;

const LoaderSubtitle = styled.div`
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.15em;
  text-transform: uppercase;
  color: #64748b;
  margin-bottom: 2rem;
`;

const LoaderText = styled(motion.div)`
  color: #0891b2;
  font-size: 0.8rem;
  font-weight: 500;
  letter-spacing: 0.1em;
  text-transform: uppercase;
`;

const ProgressBar = styled(motion.div)`
  width: 200px;
  height: 3px;
  background: #e2e8f0;
  margin-top: 1rem;
  overflow: hidden;
  border-radius: 2px;
`;

const ProgressFill = styled(motion.div)`
  height: 100%;
  background: linear-gradient(90deg, #0891b2, #06b6d4);
`;

const HUDContainer = styled.div`
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
  font-family:
    'Inter',
    -apple-system,
    system-ui,
    sans-serif;
`;

const HUDPanel = styled.div`
  position: absolute;
  top: 24px;
  left: 24px;
  background: rgba(255, 255, 255, 0.92);
  border: 1px solid rgba(0, 0, 0, 0.08);
  border-radius: 12px;
  padding: 20px 24px;
  backdrop-filter: blur(12px);
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.06);
`;

const HUDTitle = styled.h3`
  margin: 0 0 16px 0;
  font-size: 0.65rem;
  font-weight: 700;
  letter-spacing: 0.15em;
  text-transform: uppercase;
  color: #94a3b8;
`;

const HUDStat = styled.div`
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin: 8px 0;
`;

const HUDLabel = styled.span`
  font-size: 0.7rem;
  font-weight: 600;
  color: #64748b;
  text-transform: uppercase;
  letter-spacing: 0.05em;
`;

const HUDValue = styled.span<{ $color?: string }>`
  font-size: 1.1rem;
  font-weight: 700;
  color: ${props => props.$color || '#0f172a'};
  min-width: 70px;
  text-align: right;
  font-variant-numeric: tabular-nums;
`;

const HUDBadge = styled.div`
  position: absolute;
  top: 24px;
  right: 24px;
  background: #0f172a;
  color: white;
  font-size: 0.6rem;
  font-weight: 700;
  letter-spacing: 0.15em;
  text-transform: uppercase;
  padding: 10px 18px;
  border-radius: 6px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
`;

const HUDLapInfo = styled.div`
  position: absolute;
  bottom: 24px;
  left: 50%;
  transform: translateX(-50%);
  background: rgba(255, 255, 255, 0.95);
  border: 1px solid rgba(0, 0, 0, 0.08);
  border-radius: 8px;
  padding: 12px 32px;
  backdrop-filter: blur(12px);
  display: flex;
  gap: 32px;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.08);
`;

const LapItem = styled.div`
  text-align: center;
`;

const LapLabel = styled.div`
  font-size: 0.55rem;
  font-weight: 600;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  color: #94a3b8;
  margin-bottom: 2px;
`;

const LapValue = styled.div`
  font-size: 1.4rem;
  font-weight: 800;
  color: #0f172a;
  letter-spacing: -0.02em;
`;

// Reusable vectors for chase camera
const _targetPos = new THREE.Vector3();
const _lookAtPos = new THREE.Vector3();
const _velDir = new THREE.Vector3();
const _chaseOffset = new THREE.Vector3();

// ============= CHASE CAMERA =============

function ChaseCamera({ sab }: { sab: SharedArrayBuffer | null }) {
  const { camera } = useThree();
  const lookAtPosition = useRef(new THREE.Vector3(0, 3, 0));

  useFrame(() => {
    if (!sab) return;

    const stateView = new Float32Array(
      sab,
      SAB_OFFSETS.DRONE_STATE_A,
      DRONE_CONSTANTS.MAX_DRONES * (DRONE_CONSTANTS.STRIDE / 4)
    );

    // Follow hero drone (index 0)
    const px = stateView[0];
    const py = stateView[1];
    const pz = stateView[2];
    const vx = stateView[4];
    const vz = stateView[6];

    if (px === 0 && py === 0 && pz === 0) return;

    // Calculate chase offset (behind and above)
    const speed = Math.sqrt(vx * vx + vz * vz);
    if (speed > 0.1) {
      _velDir.set(vx, 0, vz).normalize();
    } else {
      _velDir.set(0, 0, -1);
    }

    // Chase position: behind drone
    _chaseOffset.copy(_velDir).multiplyScalar(-15);
    _chaseOffset.y = 8;

    _targetPos.set(px + _chaseOffset.x, py + _chaseOffset.y, pz + _chaseOffset.z);

    // Smooth follow - increase speed to catch fast drones
    camera.position.lerp(_targetPos, 0.08);

    // Look ahead of drone
    _lookAtPos.set(px + _velDir.x * 12, py + 2.0, pz + _velDir.z * 12);
    lookAtPosition.current.lerp(_lookAtPos, 0.15);

    camera.lookAt(lookAtPosition.current);
  });

  return null;
}

// ============= TELEMETRY HUD =============

function TelemetryHUD({ sab }: { sab: SharedArrayBuffer }) {
  const [telemetry, setTelemetry] = useState({
    altitude: 0,
    speed: 0,
    gate: 0,
  });

  useEffect(() => {
    if (!sab) return;

    const stateView = new Float32Array(
      sab,
      SAB_OFFSETS.DRONE_STATE_A,
      DRONE_CONSTANTS.MAX_DRONES * (DRONE_CONSTANTS.STRIDE / 4)
    );

    const interval = setInterval(() => {
      const py = stateView[1];
      const vx = stateView[4];
      const vy = stateView[5];
      const vz = stateView[6];
      const speed = Math.sqrt(vx * vx + vy * vy + vz * vz);
      const gate = Math.floor(stateView[25]) % 8;

      setTelemetry({ altitude: py, speed, gate });
    }, 100);

    return () => clearInterval(interval);
  }, [sab]);

  return (
    <HUDContainer>
      <HUDPanel>
        <HUDTitle>Lead Drone</HUDTitle>
        <HUDStat>
          <HUDLabel>Alt</HUDLabel>
          <HUDValue>{telemetry.altitude.toFixed(1)}m</HUDValue>
        </HUDStat>
        <HUDStat>
          <HUDLabel>Speed</HUDLabel>
          <HUDValue $color="#0891b2">{telemetry.speed.toFixed(1)} m/s</HUDValue>
        </HUDStat>
        <HUDStat>
          <HUDLabel>Gate</HUDLabel>
          <HUDValue $color="#ea580c">{telemetry.gate + 1}/8</HUDValue>
        </HUDStat>
      </HUDPanel>

      <HUDBadge>AUTONOMOUS</HUDBadge>

      <HUDLapInfo>
        <LapItem>
          <LapLabel>Drones</LapLabel>
          <LapValue>8</LapValue>
        </LapItem>
        <LapItem>
          <LapLabel>Circuit</LapLabel>
          <LapValue>ALPHA</LapValue>
        </LapItem>
        <LapItem>
          <LapLabel>Physics</LapLabel>
          <LapValue>250Hz</LapValue>
        </LapItem>
      </HUDLapInfo>
    </HUDContainer>
  );
}

// ============= MAIN COMPONENT =============

export default function GrandPrix() {
  const sab = useSystemStore(s => s.sab);
  const status = useSystemStore(s => s.status);
  const [ready, setReady] = useState(false);
  const [loadingStep, setLoadingStep] = useState('Waiting for Kernel...');
  const [error, setError] = useState<string | null>(null);
  const initializedRef = useRef(false);

  useEffect(() => {
    if (!sab || status !== 'ready' || initializedRef.current) return;

    const init = async () => {
      initializedRef.current = true;
      setLoadingStep('Discovering Drone Units...');
      console.log('[GrandPrix] System ready. Starting initialization...');

      try {
        await dispatch.waitUntilReady('drone');

        setLoadingStep('Starting Physics Engine...');
        const t1 = performance.now();
        await dispatch.plug('drone', 'simulation', {
          library: 'drone',
          method: 'step_physics',
          dt: 0.004,
          count: 8,
        });
        console.log(`[GrandPrix] Plug: ${(performance.now() - t1).toFixed(1)}ms`);

        setLoadingStep('Spawning Drones...');
        const t0 = performance.now();
        await dispatch.execute('drone', 'init', { count: 8 });
        console.log(`[GrandPrix] Init: ${(performance.now() - t0).toFixed(1)}ms`);

        setLoadingStep('Ready');
        setTimeout(() => setReady(true), 500);
      } catch (e) {
        console.error('[GrandPrix] Init failed:', e);
        setError(e instanceof Error ? e.message : 'Unknown error');
        initializedRef.current = false;
      }
    };
    init();
  }, [sab, status]);

  return (
    <div style={{ width: '100vw', height: '100vh', background: '#f0f9ff', position: 'relative' }}>
      <AnimatePresence>
        {!ready && (
          <LoaderContainer
            initial={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.8 }}
          >
            <LoaderLogo>Grand Prix</LoaderLogo>
            <LoaderSubtitle>Autonomous Drone Racing</LoaderSubtitle>
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
                  transition={{ duration: 3, ease: 'easeInOut' }}
                />
              </ProgressBar>
            )}
            {error && (
              <div
                style={{ marginTop: 20, cursor: 'pointer', color: '#64748b', fontSize: '0.8rem' }}
                onClick={() => window.location.reload()}
              >
                Click to Retry
              </div>
            )}
          </LoaderContainer>
        )}
      </AnimatePresence>

      <Canvas shadows dpr={[1, 2]} gl={{ antialias: true }}>
        <PerspectiveCamera makeDefault position={[0, 15, 40]} fov={55} />

        {ready && (
          <>
            <ChaseCamera sab={sab} />
            <RaceTrack />
            <DroneModel />

            {/* Post-processing for neon glow and cinematic feel */}
            <EffectComposer>
              <Bloom intensity={0.5} luminanceThreshold={0.5} luminanceSmoothing={0.5} mipmapBlur />
            </EffectComposer>

            {/* Environment fog - light but thick for depth */}
            <fog attach="fog" args={['#f0f9ff', 10, 150]} />
          </>
        )}
      </Canvas>

      {ready && sab && <TelemetryHUD sab={sab} />}
    </div>
  );
}
