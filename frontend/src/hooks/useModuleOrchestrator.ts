import { useCallback, useEffect, useRef, useState } from 'react';

interface ParticleState {
  positions: Float32Array;
  velocities: Float32Array;
  masses: Float32Array;
  moduleIds: Uint32Array;
}

interface ModuleStats {
  computeTime: number;
  scienceTime: number;
  mlTime: number;
  epoch: number;
  particleCount: number;
}

const PARTICLE_BUFFER_OFFSET = 0x200000; // 2MB offset in SAB
const PARTICLE_SIZE = 32; // bytes per particle (vec3 pos + vec3 vel + f32 mass + u32 module_id)

/**
 * Module Orchestrator Hook
 *
 * Manages SAB-based communication with WASM modules for GPU-accelerated particle physics.
 * Implements zero-copy data flow and epoch-based signaling.
 */
export function useModuleOrchestrator(particleCount: number) {
  const [stats, setStats] = useState<ModuleStats>({
    computeTime: 0,
    scienceTime: 0,
    mlTime: 0,
    epoch: 0,
    particleCount: 0,
  });

  const frameCountRef = useRef(0);
  const particleBufferRef = useRef<Float32Array | null>(null);

  // Initialize particle buffer in SAB
  const initializeParticles = useCallback(
    (initialPositions: number[], initialVelocities: number[], masses: number[]) => {
      const sab = (window as any).__INOS_SAB__;
      if (!sab || !(sab instanceof SharedArrayBuffer)) {
        console.error('[ModuleOrchestrator] SAB not available');
        return false;
      }

      try {
        // Create view into SAB particle buffer
        const buffer = new Float32Array(sab, PARTICLE_BUFFER_OFFSET, particleCount * 8);
        particleBufferRef.current = buffer;

        // Layout: [pos_x, pos_y, pos_z, vel_x, vel_y, vel_z, mass, module_id] per particle
        for (let i = 0; i < particleCount; i++) {
          const offset = i * 8;

          // Position (vec3)
          buffer[offset + 0] = initialPositions[i * 3 + 0];
          buffer[offset + 1] = initialPositions[i * 3 + 1];
          buffer[offset + 2] = initialPositions[i * 3 + 2] || 0;

          // Velocity (vec3)
          buffer[offset + 3] = initialVelocities[i * 3 + 0];
          buffer[offset + 4] = initialVelocities[i * 3 + 1];
          buffer[offset + 5] = initialVelocities[i * 3 + 2] || 0;

          // Mass
          buffer[offset + 6] = masses[i];

          // Module ID (0=ml, 1=science, 2=compute)
          buffer[offset + 7] = i % 3;
        }

        console.log(
          `[ModuleOrchestrator] ✅ Initialized ${particleCount} particles in SAB at 0x${PARTICLE_BUFFER_OFFSET.toString(16)}`
        );
        console.log(`[ModuleOrchestrator] Buffer size: ${buffer.byteLength} bytes`);

        setStats(prev => ({ ...prev, particleCount }));
        return true;
      } catch (e) {
        console.error('[ModuleOrchestrator] Failed to initialize particles:', e);
        return false;
      }
    },
    [particleCount]
  );

  // Step simulation using GPU compute module
  const step = useCallback(
    async (dt: number) => {
      const modules = (window as any).inosModules;
      if (!modules) {
        console.warn('[ModuleOrchestrator] Modules not loaded yet');
        return;
      }

      const frame = frameCountRef.current++;

      try {
        // 1. GPU Compute - N-body physics
        let computeTime = 0;
        if (modules.compute && typeof modules.compute.compute_nbody_step === 'function') {
          const t0 = performance.now();
          const result = modules.compute.compute_nbody_step(particleCount, dt);
          computeTime = performance.now() - t0;

          if (result !== 1) {
            console.warn(`[ModuleOrchestrator] Compute returned ${result}`);
          }
        }

        // 2. Science Module - Conservation validation (every 30 frames)
        let scienceTime = 0;
        if (
          frame % 30 === 0 &&
          modules.science &&
          typeof modules.science.validate_conservation === 'function'
        ) {
          const t1 = performance.now();
          const result = modules.science.validate_conservation(particleCount);
          scienceTime = performance.now() - t1;

          if (result !== 1) {
            console.warn(`[ModuleOrchestrator] Science validation returned ${result}`);
          }
        }

        // 3. ML Module - Pattern detection (every 60 frames)
        let mlTime = 0;
        if (
          frame % 60 === 0 &&
          modules.ml &&
          typeof modules.ml.detect_particle_patterns === 'function'
        ) {
          const t2 = performance.now();
          const result = modules.ml.detect_particle_patterns(particleCount);
          mlTime = performance.now() - t2;

          if (result !== 1) {
            console.warn(`[ModuleOrchestrator] ML pattern detection returned ${result}`);
          }
        }

        // Read current epoch
        const sab = (window as any).__INOS_SAB__;
        let epoch = 0;
        if (sab) {
          const flags = new Int32Array(sab, 0, 16);
          epoch = Atomics.load(flags, 7); // IDX_SYSTEM_EPOCH
        }

        // Update stats
        setStats({
          computeTime,
          scienceTime,
          mlTime,
          epoch,
          particleCount,
        });

        // Log every 60 frames
        if (frame % 60 === 0) {
          console.log(`[ModuleOrchestrator] Frame ${frame}:`, {
            compute: `${computeTime.toFixed(2)}ms`,
            science: scienceTime > 0 ? `${scienceTime.toFixed(2)}ms` : 'skipped',
            ml: mlTime > 0 ? `${mlTime.toFixed(2)}ms` : 'skipped',
            epoch,
          });
        }
      } catch (e) {
        console.error('[ModuleOrchestrator] Step failed:', e);
      }
    },
    [particleCount]
  );

  // Get current particle state from SAB
  const getParticles = useCallback((): ParticleState | null => {
    const buffer = particleBufferRef.current;
    if (!buffer) {
      return null;
    }

    // Extract data from interleaved buffer
    const positions = new Float32Array(particleCount * 3);
    const velocities = new Float32Array(particleCount * 3);
    const masses = new Float32Array(particleCount);
    const moduleIds = new Uint32Array(particleCount);

    for (let i = 0; i < particleCount; i++) {
      const offset = i * 8;

      positions[i * 3 + 0] = buffer[offset + 0];
      positions[i * 3 + 1] = buffer[offset + 1];
      positions[i * 3 + 2] = buffer[offset + 2];

      velocities[i * 3 + 0] = buffer[offset + 3];
      velocities[i * 3 + 1] = buffer[offset + 4];
      velocities[i * 3 + 2] = buffer[offset + 5];

      masses[i] = buffer[offset + 6];
      moduleIds[i] = buffer[offset + 7];
    }

    return { positions, velocities, masses, moduleIds };
  }, [particleCount]);

  // Check if modules are ready
  const isReady = useCallback(() => {
    const sab = (window as any).__INOS_SAB__;
    const modules = (window as any).inosModules;
    return !!(sab && modules && particleBufferRef.current);
  }, []);

  return {
    initializeParticles,
    step,
    getParticles,
    isReady,
    stats,
  };
}
