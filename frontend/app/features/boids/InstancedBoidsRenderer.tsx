import { useFrame } from '@react-three/fiber';
import { useRef, useMemo, useEffect } from 'react';
import * as THREE from 'three';
import { useSystemStore } from '../../../src/store/system';
import { dispatch } from '../../../src/wasm/dispatch';
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

interface Props {
  variant?: 'bird' | 'fish';
}

export default function InstancedBoidsRenderer({ variant = 'bird' }: Props) {
  const status = useSystemStore(s => s.status);

  // Refs for each part's instanced mesh
  const bodiesRef = useRef<THREE.InstancedMesh>(null);
  const headsRef = useRef<THREE.InstancedMesh>(null);
  const beaksRef = useRef<THREE.InstancedMesh>(null);
  const leftWingRef = useRef<THREE.InstancedMesh>(null);
  const leftWingTipRef = useRef<THREE.InstancedMesh>(null);
  const rightWingRef = useRef<THREE.InstancedMesh>(null);
  const rightWingTipRef = useRef<THREE.InstancedMesh>(null);
  const tailsRef = useRef<THREE.InstancedMesh>(null);
  const flagsRef = useRef<Int32Array | null>(null);

  // Epoch-based stall resilience: track last seen epoch to detect stalls
  const lastEpochRef = useRef<number>(-1);

  // Shared geometries
  const geometries = useMemo(() => {
    // Helper to orient geometries
    const alignGeo = (
      geo: THREE.BufferGeometry,
      rotX = 0,
      rotY = 0,
      rotZ = 0,
      scale = [1, 1, 1]
    ) => {
      if (rotX !== 0) geo.rotateX(rotX);
      if (rotY !== 0) geo.rotateY(rotY);
      if (rotZ !== 0) geo.rotateZ(rotZ);
      geo.scale(scale[0], scale[1], scale[2]);
      return geo;
    };

    if (variant === 'fish') {
      return {
        // Body: Fusiform shape (scaled sphere/capsule)
        body: alignGeo(new THREE.CapsuleGeometry(0.1, 0.5, 4, 8), Math.PI / 2, 0, 0, [1, 1, 1]), // Horizontal capsule
        // Head: Merged into body mostly, but can be a small snout
        head: alignGeo(new THREE.ConeGeometry(0.08, 0.2, 5), Math.PI / 2, 0, 0, [1, 1, 1]), // Pointy nose
        // Beak: Hidden or very small
        beak: alignGeo(new THREE.BoxGeometry(0.01, 0.01, 0.01), 0, 0, 0),
        // Fins (Side): Vertical-ish planes
        wing: alignGeo(new THREE.PlaneGeometry(0.3, 0.15, 2, 2), 0, Math.PI / 4, 0, [1, 1, 1]),
        wingTip: alignGeo(new THREE.PlaneGeometry(0.1, 0.1, 1, 1), 0, 0, 0, [0.01, 0.01, 0.01]), // Hide tips
        // Tail: Vertical Fin
        tail: alignGeo(new THREE.PlaneGeometry(0.3, 0.4, 3, 2), 0, Math.PI / 2, 0, [1, 1, 1]),
      };
    }

    return {
      // Body: Slim cylinder, wireframe friendly (6 segments = hexagon)
      body: alignGeo(new THREE.CylinderGeometry(0.025, 0.05, 0.45, 6), 0, 0, 0),
      // Head: Smaller, geodesic (Icosahedron)
      head: alignGeo(new THREE.IcosahedronGeometry(0.06, 0), 0, 0, 0),
      // Beak: Sharp cone
      beak: alignGeo(new THREE.ConeGeometry(0.02, 0.15, 4), 0, 0, 0),
      // Wing: Segmented plane for wireframe structure (3x2 segments)
      wing: alignGeo(new THREE.PlaneGeometry(0.45, 0.25, 3, 2), 0, 0, 0),
      wingTip: alignGeo(new THREE.PlaneGeometry(0.38, 0.15, 3, 1), 0, 0, 0),
      // Tail: Flattened Cone (Fan shape).
      // Rotated 90deg X to lie flat (horizontal fan) instead of pointing up.
      // Tip points towards body (-Z or +Z depending on logic, aligned to standard Z-forward).
      // Scale Y (thickness) down to make it a flat fan.
      tail: alignGeo(new THREE.ConeGeometry(0.12, 0.35, 5), Math.PI / 2, 0, 0, [1, 0.05, 1]),
    };
  }, [variant]);

  // Materials with disposal
  const materials = useMemo(
    () => ({
      body: new THREE.MeshBasicMaterial({ wireframe: true, transparent: true, opacity: 0.6 }),
      head: new THREE.MeshBasicMaterial({ wireframe: true, transparent: true, opacity: 0.6 }),
      beak: new THREE.MeshBasicMaterial({ wireframe: true, transparent: true, opacity: 0.6 }),
      wing: new THREE.MeshBasicMaterial({
        side: THREE.DoubleSide,
        transparent: true,
        opacity: 0.35,
        wireframe: true,
      }),
      wingTip: new THREE.MeshBasicMaterial({
        side: THREE.DoubleSide,
        transparent: true,
        opacity: 0.2,
        wireframe: true,
      }),
      tail: new THREE.MeshBasicMaterial({
        side: THREE.DoubleSide,
        transparent: true,
        opacity: 0.45,
        wireframe: true,
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

  // Shared colors palette (Realistic Wireframes: Mid-Dark Slate/Stone)
  // Lightened by one shade (e.g. 700 -> 600) to be softer but still visible
  const palette = useMemo(
    () => [
      new THREE.Color('#475569'), // slate-600
      new THREE.Color('#57534e'), // stone-600
      new THREE.Color('#71717a'), // zinc-500
      new THREE.Color('#4b5563'), // gray-600
      new THREE.Color('#0284c7'), // sky-600 (accent)
    ],
    []
  );

  const sharedColors = useMemo(
    () => ({
      wing: new THREE.Color('#737373'), // neutral-500
      tail: new THREE.Color('#64748b'), // slate-500
    }),
    []
  );

  // Initialize boids population once when system is ready
  useEffect(() => {
    if (status === 'ready') {
      console.log('[BoidsFlock] Initializing boids population');
      dispatch.execute('boids', 'init_population', { bird_count: CONFIG.BIRD_COUNT });
    }
  }, [status]);

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
    // Wait for system to be ready (dispatcher initialized)
    if (status !== 'ready') return;

    const sab = (window as any).__INOS_SAB__;
    if (!sab) return;

    // 1. ALWAYS dispatch physics and matrix generation (they update SAB and flip epochs)
    dispatch
      .execute('boids', 'step_physics', {
        bird_count: CONFIG.BIRD_COUNT,
        dt: delta,
      })
      .catch(err => console.error('[BoidsRenderer] Physics step failed:', err));

    dispatch
      .execute('math', 'compute_instance_matrices', {
        count: CONFIG.BIRD_COUNT,
        source_offset: CONFIG.SAB_OFFSET,
        pivots: [],
      })
      .catch(err => console.error('[BoidsRenderer] Matrix compute failed:', err));

    // 2. Check if matrix epoch changed (indicates new data available)
    if (!flagsRef.current || flagsRef.current.buffer !== sab) {
      flagsRef.current = new Int32Array(sab, 0, 16);
    }
    const flags = flagsRef.current;
    const matrixEpoch = Atomics.load(flags, IDX_MATRIX_EPOCH);

    // ═══════════════════════════════════════════════════════════════════════════
    // STALL RESILIENCE: If epoch unchanged, skip matrix update (reuse GPU buffer).
    // Dispatch ALWAYS runs (to drive physics forward), but we only upload to GPU
    // when new data is ready. This prevents redundant GPU uploads, not stalls.
    // ═══════════════════════════════════════════════════════════════════════════
    if (matrixEpoch === lastEpochRef.current) {
      return; // GPU already has latest data, skip redundant upload
    }
    lastEpochRef.current = matrixEpoch;

    // 3. Determine which ping-pong buffer to read from
    const isBufferA = Number(matrixEpoch) % 2 === 0;
    const matrixBase = isBufferA ? OFFSET_MATRIX_BUFFER_A : OFFSET_MATRIX_BUFFER_B;

    // 4. Zero-copy pointer swap for each bird part
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

        // Architecture: Zero-Copy Pointer Swap (no memcpy between CPU stages)
        const sabView = getArenaView(sab, matrixOffset, CONFIG.BIRD_COUNT * 64);

        // Final GPU upload (unavoidable - WebGPU buffer ownership semantics)
        (ref.current.instanceMatrix as any).array = sabView;
        ref.current.instanceMatrix.needsUpdate = true;
      }
    });
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
    </>
  );
}
