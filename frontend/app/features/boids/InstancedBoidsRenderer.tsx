import { useFrame } from '@react-three/fiber';
import { useRef, useMemo, useEffect } from 'react';
import * as THREE from 'three';
import { useSystemStore } from '../../../src/store/system';
import { dispatch } from '../../../src/wasm/dispatch';
import LeaderMarker from './LeaderMarker';
import {
  OFFSET_BIRD_BUFFER_A,
  OFFSET_MATRIX_BUFFER_A,
  OFFSET_MATRIX_BUFFER_B,
  IDX_MATRIX_EPOCH,
  getLayoutConfig,
  type ResourceTier,
} from '../../../src/wasm/layout';
import { getArenaView } from '../scenes/SceneWrapper';

// Get tier from window or default to 'light'
const tier: ResourceTier =
  (typeof window !== 'undefined' && (window as any).__INOS_TIER__) || 'light';
const tierConfig = getLayoutConfig(tier);

const CONFIG = {
  BIRD_COUNT: tierConfig.recommended,
  SAB_OFFSET: OFFSET_BIRD_BUFFER_A,
  BYTES_PER_BIRD: 236,
};

export default function InstancedBoidsRenderer() {
  const { moduleExports } = useSystemStore();
  const moduleExportsRef = useRef(moduleExports);
  moduleExportsRef.current = moduleExports;

  // Refs for each part's instanced mesh
  const bodiesRef = useRef<THREE.InstancedMesh>(null);
  const headsRef = useRef<THREE.InstancedMesh>(null);
  const beaksRef = useRef<THREE.InstancedMesh>(null);
  const leftWingRef = useRef<THREE.InstancedMesh>(null);
  const leftWingTipRef = useRef<THREE.InstancedMesh>(null);
  const rightWingRef = useRef<THREE.InstancedMesh>(null);
  const rightWingTipRef = useRef<THREE.InstancedMesh>(null);
  const tailsRef = useRef<THREE.InstancedMesh>(null);

  // Shared geometries
  const geometries = useMemo(
    () => ({
      body: new THREE.CylinderGeometry(0.03, 0.06, 0.45, 8),
      head: new THREE.BoxGeometry(0.12, 0.12, 0.12),
      beak: new THREE.ConeGeometry(0.03, 0.22, 6),
      wing: new THREE.PlaneGeometry(0.45, 0.18),
      wingTip: new THREE.PlaneGeometry(0.38, 0.12),
      tail: new THREE.PlaneGeometry(0.08, 0.3),
    }),
    []
  );

  // Materials with disposal
  const materials = useMemo(
    () => ({
      body: new THREE.MeshBasicMaterial(),
      head: new THREE.MeshBasicMaterial(),
      beak: new THREE.MeshBasicMaterial(),
      wing: new THREE.MeshBasicMaterial({
        side: THREE.DoubleSide,
        transparent: true,
        opacity: 0.4,
      }),
      wingTip: new THREE.MeshBasicMaterial({
        side: THREE.DoubleSide,
        transparent: true,
        opacity: 0.2,
      }),
      tail: new THREE.MeshBasicMaterial({
        side: THREE.DoubleSide,
        transparent: true,
        opacity: 0.5,
      }),
    }),
    []
  );

  // Disposal - CRITICAL: InstancedMesh instanceMatrix/instanceColor buffers are NOT
  // part of geometry and require separate disposal to prevent WebGL memory leaks
  useEffect(() => {
    return () => {
      console.log('[BoidsFlock] Disposing resources...');

      // Dispose geometries and materials
      Object.values(geometries).forEach(g => g.dispose());
      Object.values(materials).forEach(m => m.dispose());

      // Dispose InstancedMesh instances (important for WebGL buffer cleanup)
      const meshRefs = [
        bodiesRef,
        headsRef,
        beaksRef,
        leftWingRef,
        leftWingTipRef,
        rightWingRef,
        rightWingTipRef,
        tailsRef,
      ];

      meshRefs.forEach(ref => {
        if (ref.current) {
          // Dispose the mesh (releases WebGL resources)
          ref.current.dispose();
        }
      });
    };
  }, [geometries, materials]);

  // Shared colors palette (Optimized: No per-bird color objects)
  const palette = useMemo(
    () => [
      new THREE.Color('#6d28d9'),
      new THREE.Color('#ec4899'),
      new THREE.Color('#10b981'),
      new THREE.Color('#f59e0b'),
    ],
    []
  );

  const sharedColors = useMemo(
    () => ({
      wing: new THREE.Color('#555555'),
      tail: new THREE.Color('#888888'),
    }),
    []
  );

  // Initialize ONCE
  useEffect(() => {
    if (moduleExports?.compute) {
      console.log('[BoidsFlock] Initializing boids population via dispatcher');
      dispatch.execute('boids', 'init_population', { bird_count: CONFIG.BIRD_COUNT });
    }
  }, []);

  useEffect(() => {
    const refs = [
      bodiesRef,
      headsRef,
      beaksRef,
      leftWingRef,
      leftWingTipRef,
      rightWingRef,
      rightWingTipRef,
      tailsRef,
    ];
    refs.forEach(ref => {
      if (ref.current) {
        for (let i = 0; i < CONFIG.BIRD_COUNT; i++) {
          if (ref === leftWingRef || ref === rightWingRef) {
            ref.current.setColorAt(i, sharedColors.wing);
          } else if (ref === tailsRef) {
            ref.current.setColorAt(i, sharedColors.tail);
          } else {
            ref.current.setColorAt(i, palette[i % palette.length]);
          }
        }
        ref.current.instanceColor!.needsUpdate = true;
      }
    });
  }, [palette, sharedColors]);

  useFrame((_, delta) => {
    // 1. Run physics step in WASM via Dispatcher
    if (moduleExports?.compute) {
      dispatch.execute('boids', 'step_physics', {
        bird_count: CONFIG.BIRD_COUNT,
        dt: delta, // Full refresh rate timing
      });

      // 2. Offload MATRIX MATH to MathUnit (Zero-Copy)
      // Destination is determined by the math unit's internal ping-pong logic
      dispatch.execute('math', 'compute_instance_matrices', {
        count: CONFIG.BIRD_COUNT,
        source_offset: CONFIG.SAB_OFFSET,
        pivots: [], // Hardcoded in WASM for performance
      });

      // 3. Update InstancedMesh matrices from SAB views
      const sab = (window as any).__INOS_SAB__;
      if (!sab) return;

      // Determine active matrix buffer from epoch at IDX_MATRIX_EPOCH
      const flags = new Int32Array(sab, 0, 16);
      const matrixEpoch = Atomics.load(flags, IDX_MATRIX_EPOCH);
      const isBufferA = matrixEpoch % 2 === 0;

      // Use layout constants for buffer offsets
      const matrixBase = isBufferA ? OFFSET_MATRIX_BUFFER_A : OFFSET_MATRIX_BUFFER_B;

      const instances = [
        bodiesRef,
        headsRef,
        beaksRef,
        leftWingRef,
        leftWingTipRef,
        rightWingRef,
        rightWingTipRef,
        tailsRef,
      ];

      instances.forEach((ref, partIdx) => {
        if (ref.current) {
          // OFFSET = matrixBase + partIdx * count * 64
          const matrixOffset = matrixBase + partIdx * CONFIG.BIRD_COUNT * 64;

          // Architecture: Zero-Copy Pointer Swap
          // Instead of .array.set(sabView) which COPIES data, we re-bind the BufferAttribute
          // to point directly to the shared memory view for this frame.
          const sabView = getArenaView(sab, matrixOffset, CONFIG.BIRD_COUNT * 64);

          // Pointer Swap Optimization:
          (ref.current.instanceMatrix as any).array = sabView;
          ref.current.instanceMatrix.needsUpdate = true;
        }
      });
    }
  });

  return (
    <>
      <instancedMesh ref={bodiesRef} args={[geometries.body, materials.body, CONFIG.BIRD_COUNT]} />
      <instancedMesh ref={headsRef} args={[geometries.head, materials.head, CONFIG.BIRD_COUNT]} />
      <instancedMesh ref={beaksRef} args={[geometries.beak, materials.beak, CONFIG.BIRD_COUNT]} />
      <instancedMesh
        ref={leftWingRef}
        args={[geometries.wing, materials.wing, CONFIG.BIRD_COUNT]}
      />
      <instancedMesh
        ref={leftWingTipRef}
        args={[geometries.wingTip, materials.wingTip, CONFIG.BIRD_COUNT]}
      />
      <instancedMesh
        ref={rightWingRef}
        args={[geometries.wing, materials.wing, CONFIG.BIRD_COUNT]}
      />
      <instancedMesh
        ref={rightWingTipRef}
        args={[geometries.wingTip, materials.wingTip, CONFIG.BIRD_COUNT]}
      />
      <instancedMesh ref={tailsRef} args={[geometries.tail, materials.tail, CONFIG.BIRD_COUNT]} />

      {/* Leader HUD - Approach B: Minimalist Three.js Marker (Zero-Copy) */}
      {(window as any).__INOS_SAB__ && (
        <LeaderMarker sab={(window as any).__INOS_SAB__} birdIndex={0} offset={CONFIG.SAB_OFFSET} />
      )}
    </>
  );
}
