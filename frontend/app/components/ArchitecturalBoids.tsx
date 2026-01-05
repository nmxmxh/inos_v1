import { Canvas, useFrame } from '@react-three/fiber';
import { useRef, useMemo, Suspense, useEffect } from 'react';
import * as THREE from 'three';
import { useSystemStore } from '../../src/store/system';
import { dispatch } from '../../src/wasm/dispatch';

const CONFIG = {
  BIRD_COUNT: 1000,
  SAB_OFFSET: 0x400000,
  BYTES_PER_BIRD: 236,
};

function InstancedBoidsRenderer() {
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

  // Disposal
  useEffect(() => {
    return () => {
      console.log('[BoidsFlock] Disposing resources...');
      Object.values(geometries).forEach(g => g.dispose());
      Object.values(materials).forEach(m => m.dispose());
    };
  }, [geometries, materials]);

  // Shared colors palette
  const colors = useMemo(() => {
    const palette = ['#6d28d9', '#ec4899', '#10b981', '#f59e0b'];
    return Array.from(
      { length: CONFIG.BIRD_COUNT },
      (_, i) => new THREE.Color(palette[i % palette.length])
    );
  }, []);

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
            ref.current.setColorAt(i, colors[i]);
          }
        }
        ref.current.instanceColor!.needsUpdate = true;
      }
    });
  }, [colors, sharedColors]);

  useFrame((_, delta) => {
    // 1. Run physics step in WASM via Dispatcher
    if (moduleExports?.compute) {
      dispatch.execute('boids', 'step_physics', {
        bird_count: CONFIG.BIRD_COUNT,
        dt: delta,
      });

      // 2. Offload MATRIX MATH to MathUnit (Zero-Copy)
      // Base data at 0x400000, Output matrices start at 0x500000
      const MATRIX_BASE = 0x500000;
      dispatch.execute('math', 'compute_instance_matrices', {
        count: CONFIG.BIRD_COUNT,
        source_offset: CONFIG.SAB_OFFSET,
        target_offset: MATRIX_BASE,
        pivots: [], // Hardcoded in WASM for performance
      });

      // 3. Update InstancedMesh matrices from SAB views
      const sab = (window as any).__INOS_SAB__;
      if (!sab) return;

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
          const matrixOffset = MATRIX_BASE + partIdx * CONFIG.BIRD_COUNT * 64;
          // Zero-copy: set the underlying attribute array from a view of the SAB
          const sabView = new Float32Array(sab, matrixOffset, CONFIG.BIRD_COUNT * 16);
          ref.current.instanceMatrix.array.set(sabView);
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
    </>
  );
}

export default function ArchitecturalBoids() {
  return (
    <div
      style={{
        width: '100%',
        height: '100%',
        position: 'fixed',
        top: 0,
        left: 0,
        zIndex: 0,
        pointerEvents: 'none',
      }}
    >
      {/* <DebugHUD title="INOS ZERO-COPY ENGINE" color="#6d28d9" stats={stats} /> */}
      <Suspense fallback={null}>
        <Canvas
          camera={{ position: [0, 8, 30], fov: 45 }}
          style={{ background: 'transparent' }}
          dpr={[1, 2]}
          gl={{
            alpha: true,
            antialias: true,
            powerPreference: 'high-performance',
          }}
        >
          <ambientLight intensity={0.6} />
          <pointLight position={[15, 15, 15]} intensity={0.8} />
          <InstancedBoidsRenderer />
          <gridHelper args={[80, 10, '#330066', '#110022']} position={[0, -10, 0]} />
        </Canvas>
      </Suspense>
    </div>
  );
}
