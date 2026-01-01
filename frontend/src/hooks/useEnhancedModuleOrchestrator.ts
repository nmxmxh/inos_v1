import { useCallback, useEffect, useRef, useState } from 'react';

interface EnhancedParticle {
  position: [number, number, number];
  velocity: [number, number, number];
  acceleration: [number, number, number];
  mass: number;
  radius: number;
  color: [number, number, number, number];
  temperature: number;
  luminosity: number;
  particleType: number; // 0=normal, 1=star, 2=black hole, 3=dark matter
  lifetime: number;
  angularVelocity: [number, number, number];
}

interface SimulationParams {
  G: number;
  dt: number;
  particleCount: number;
  softening: number;
  forceLaw: number; // 0=Newtonian, 1=Plummer, 2=Cubic, 3=Logarithmic
  darkMatterFactor: number;
  cosmicExpansion: number;
  enableCollisions: boolean;
  mergeThreshold: number;
  restitution: number;
  tidalForces: boolean;
  dragCoefficient: number;
  turbulenceStrength: number;
  turbulenceScale: number;
  magneticStrength: number;
  radiationPressure: number;
  universeRadius: number;
  backgroundDensity: number;
  time: number;
}

interface ModuleStats {
  computeTime: number;
  scienceTime: number;
  mlTime: number;
  epoch: number;
  particleCount: number;
}

const PARTICLE_BUFFER_OFFSET = 0x200000; // 2MB offset in SAB
const PARTICLE_SIZE = 88; // 22 floats × 4 bytes = 88 bytes per particle
const PARAMS_OFFSET = 0x300000; // 3MB offset for simulation parameters

/**
 * Enhanced Module Orchestrator Hook
 *
 * Manages SAB-based communication with WASM modules for GPU-accelerated particle physics
 * with advanced features: multiple force laws, particle types, collisions, visual effects
 */
export function useEnhancedModuleOrchestrator(
  particleCount: number,
  initialParams: Partial<SimulationParams> = {}
) {
  const [stats, setStats] = useState<ModuleStats>({
    computeTime: 0,
    scienceTime: 0,
    mlTime: 0,
    epoch: 0,
    particleCount: 0,
  });

  const [params, setParams] = useState<SimulationParams>({
    G: 5.0,
    dt: 0.016,
    particleCount,
    softening: 15.0,
    forceLaw: 0, // Newtonian by default
    darkMatterFactor: 0.5,
    cosmicExpansion: 0.0,
    enableCollisions: true,
    mergeThreshold: 1.2,
    restitution: 0.3,
    tidalForces: true,
    dragCoefficient: 0.01,
    turbulenceStrength: 0.1,
    turbulenceScale: 0.05,
    magneticStrength: 0.05,
    radiationPressure: 0.01,
    universeRadius: 1000.0,
    backgroundDensity: 0.1,
    time: 0,
    ...initialParams,
  });

  const frameCountRef = useRef(0);
  const particleBufferRef = useRef<Float32Array | null>(null);
  const timeRef = useRef(0);

  // Initialize enhanced particle buffer in SAB
  const initializeParticles = useCallback(
    (
      initialPositions: number[],
      initialVelocities: number[],
      masses: number[],
      particleTypes?: number[]
    ) => {
      const sab = (window as any).__INOS_SAB__;
      if (!sab || !(sab instanceof SharedArrayBuffer)) {
        console.error('[EnhancedOrchestrator] SAB not available');
        return false;
      }

      try {
        // Create view into SAB particle buffer (22 floats per particle)
        const buffer = new Float32Array(sab, PARTICLE_BUFFER_OFFSET, particleCount * 22);
        particleBufferRef.current = buffer;

        // Layout: position(3) + velocity(3) + acceleration(3) + mass(1) + radius(1) +
        //         color(4) + temperature(1) + luminosity(1) + type(1) + lifetime(1) + angular_vel(3)
        for (let i = 0; i < particleCount; i++) {
          const offset = i * 22;

          // Position (vec3)
          buffer[offset + 0] = initialPositions[i * 3 + 0];
          buffer[offset + 1] = initialPositions[i * 3 + 1];
          buffer[offset + 2] = initialPositions[i * 3 + 2] || 0;

          // Velocity (vec3)
          buffer[offset + 3] = initialVelocities[i * 3 + 0];
          buffer[offset + 4] = initialVelocities[i * 3 + 1];
          buffer[offset + 5] = initialVelocities[i * 3 + 2] || 0;

          // Acceleration (vec3) - initially zero
          buffer[offset + 6] = 0;
          buffer[offset + 7] = 0;
          buffer[offset + 8] = 0;

          // Mass
          buffer[offset + 9] = masses[i];

          // Radius (based on mass)
          buffer[offset + 10] = Math.pow(masses[i], 0.333) * 2;

          // Color (vec4) - will be set by temperature
          const type = particleTypes ? particleTypes[i] : i % 4;
          if (type === 2) {
            // Black hole
            buffer[offset + 11] = 0.05;
            buffer[offset + 12] = 0.05;
            buffer[offset + 13] = 0.1;
            buffer[offset + 14] = 1.0;
          } else if (type === 3) {
            // Dark matter
            buffer[offset + 11] = 0.3;
            buffer[offset + 12] = 0.1;
            buffer[offset + 13] = 0.5;
            buffer[offset + 14] = 0.3;
          } else {
            // Normal/Star
            buffer[offset + 11] = 1.0;
            buffer[offset + 12] = 1.0;
            buffer[offset + 13] = 1.0;
            buffer[offset + 14] = 1.0;
          }

          // Temperature (5000-15000K for stars)
          buffer[offset + 15] = 5000 + Math.random() * 10000;

          // Luminosity
          buffer[offset + 16] = 0.5 + Math.random() * 0.5;

          // Particle type
          buffer[offset + 17] = type;

          // Lifetime (100-1000 time units)
          buffer[offset + 18] = 100 + Math.random() * 900;

          // Angular velocity (vec3) - initially zero
          buffer[offset + 19] = 0;
          buffer[offset + 20] = 0;
          buffer[offset + 21] = 0;
        }

        console.log(
          `[EnhancedOrchestrator] ✅ Initialized ${particleCount} enhanced particles in SAB at 0x${PARTICLE_BUFFER_OFFSET.toString(16)}`
        );
        console.log(
          `[EnhancedOrchestrator] Buffer size: ${buffer.byteLength} bytes (${PARTICLE_SIZE} bytes/particle)`
        );

        // Initialize simulation with WASM module
        const modules = (window as any).inosModules;
        if (modules?.compute?.compute_init_nbody_enhanced) {
          const result = modules.compute.compute_init_nbody_enhanced(
            particleCount,
            params.forceLaw,
            params.enableCollisions ? 1 : 0
          );
          console.log(`[EnhancedOrchestrator] WASM init result: ${result}`);
        }

        setStats(prev => ({ ...prev, particleCount }));
        return true;
      } catch (e) {
        console.error('[EnhancedOrchestrator] Failed to initialize particles:', e);
        return false;
      }
    },
    [particleCount, params.forceLaw, params.enableCollisions]
  );

  // Step simulation using enhanced GPU compute
  const step = useCallback(
    async (dt: number) => {
      const modules = (window as any).inosModules;
      if (!modules) {
        console.warn('[EnhancedOrchestrator] Modules not loaded yet');
        return;
      }

      const frame = frameCountRef.current++;
      timeRef.current += dt;

      try {
        // Update time parameter in SAB
        const sab = (window as any).__INOS_SAB__;
        if (sab) {
          // Time is at index 18 in params (18 * 4 = 72 bytes offset)
          const paramsView = new Float32Array(sab, PARAMS_OFFSET, 20);
          paramsView[18] = timeRef.current;
        }

        // 1. GPU Compute - Enhanced N-body physics
        let computeTime = 0;
        if (modules.compute && typeof modules.compute.compute_nbody_step_enhanced === 'function') {
          const t0 = performance.now();
          const result = modules.compute.compute_nbody_step_enhanced(particleCount, dt);
          computeTime = performance.now() - t0;

          if (result !== 1) {
            console.warn(`[EnhancedOrchestrator] Compute returned ${result}`);
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
            console.warn(`[EnhancedOrchestrator] Science validation returned ${result}`);
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
            console.warn(`[EnhancedOrchestrator] ML pattern detection returned ${result}`);
          }
        }

        // Read current epoch
        const epoch = getEpoch();

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
          console.log(`[EnhancedOrchestrator] Frame ${frame}:`, {
            compute: `${computeTime.toFixed(2)}ms`,
            science: scienceTime > 0 ? `${scienceTime.toFixed(2)}ms` : 'skipped',
            ml: mlTime > 0 ? `${mlTime.toFixed(2)}ms` : 'skipped',
            epoch,
            time: timeRef.current.toFixed(2),
          });
        }
      } catch (e) {
        console.error('[EnhancedOrchestrator] Step failed:', e);
      }
    },
    [particleCount]
  );

  // Get current particle state from SAB
  const getParticles = useCallback((): EnhancedParticle[] => {
    const buffer = particleBufferRef.current;
    if (!buffer) {
      return [];
    }

    const particles: EnhancedParticle[] = [];

    for (let i = 0; i < particleCount; i++) {
      const offset = i * 22;

      particles.push({
        position: [buffer[offset + 0], buffer[offset + 1], buffer[offset + 2]],
        velocity: [buffer[offset + 3], buffer[offset + 4], buffer[offset + 5]],
        acceleration: [buffer[offset + 6], buffer[offset + 7], buffer[offset + 8]],
        mass: buffer[offset + 9],
        radius: buffer[offset + 10],
        color: [buffer[offset + 11], buffer[offset + 12], buffer[offset + 13], buffer[offset + 14]],
        temperature: buffer[offset + 15],
        luminosity: buffer[offset + 16],
        particleType: buffer[offset + 17],
        lifetime: buffer[offset + 18],
        angularVelocity: [buffer[offset + 19], buffer[offset + 20], buffer[offset + 21]],
      });
    }

    return particles;
  }, [particleCount]);

  // Update simulation parameter
  const updateParam = useCallback((paramName: keyof SimulationParams, value: number) => {
    setParams(prev => ({ ...prev, [paramName]: value }));

    // Update in SAB via WASM module
    const modules = (window as any).inosModules;
    if (modules?.compute?.compute_set_sim_params) {
      const paramIndex = getParamIndex(paramName);
      if (paramIndex !== -1) {
        modules.compute.compute_set_sim_params(paramIndex, value);
      }
    }
  }, []);

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
    updateParam,
    isReady,
    stats,
    params,
  };
}

// Helper to get parameter index for SAB updates
function getParamIndex(paramName: string): number {
  const paramMap: Record<string, number> = {
    G: 0,
    dt: 1,
    particleCount: 2,
    softening: 3,
    forceLaw: 4,
    darkMatterFactor: 5,
    cosmicExpansion: 6,
    enableCollisions: 7,
    mergeThreshold: 8,
    restitution: 9,
    tidalForces: 10,
    dragCoefficient: 11,
    turbulenceStrength: 12,
    turbulenceScale: 13,
    magneticStrength: 14,
    radiationPressure: 15,
    universeRadius: 16,
    backgroundDensity: 17,
    time: 18,
  };
  return paramMap[paramName] ?? -1;
}

// Helper to read epoch from SAB
function getEpoch(): number {
  const sab = (window as any).__INOS_SAB__;
  if (!sab) return 0;

  const flags = new Int32Array(sab, 0, 16);
  return Atomics.load(flags, 7); // IDX_SYSTEM_EPOCH
}
