import { Canvas, useFrame } from '@react-three/fiber';
import { useRef, useMemo, Suspense, useEffect } from 'react';
import * as THREE from 'three';
import { useSystemStore } from '../../src/store/system';

const CONFIG = {
  BIRD_COUNT: 1000,
  SAB_OFFSET: 0x400000,
  BYTES_PER_BIRD: 232,
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

  // Shared dummy for math
  const dummy = useMemo(() => new THREE.Object3D(), []);
  const partDummy = useMemo(() => new THREE.Object3D(), []);
  const matrix = useMemo(() => new THREE.Matrix4(), []);
  const m2 = useMemo(() => new THREE.Matrix4(), []); // Scratch 2
  const m3 = useMemo(() => new THREE.Matrix4(), []); // Scratch 3

  // Persistent view for SAB reading
  const birdViewRef = useRef<Float32Array | null>(null);

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

  // Initialize ONCE - use ref to prevent re-init on moduleExports changes
  useEffect(() => {
    const exports = moduleExportsRef.current;
    if (exports && exports.compute?.compute_boids_init) {
      console.log('[BoidsFlock] Initializing boids population');
      exports.compute.compute_boids_init(CONFIG.BIRD_COUNT);
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
    // 1. Run physics step in WASM
    if (moduleExports && moduleExports.compute?.compute_boids_step) {
      moduleExports.compute.compute_boids_step(CONFIG.BIRD_COUNT, delta);
    }

    // 2. Read state from SAB and update matrices
    const sab = (window as any).__INOS_SAB__;
    if (!sab) return;

    // Initialize or reuse persistent view
    if (!birdViewRef.current || birdViewRef.current.buffer !== sab) {
      // Create a view of the entire relevant SAB space
      birdViewRef.current = new Float32Array(sab);
    }
    const view = birdViewRef.current;

    for (let i = 0; i < CONFIG.BIRD_COUNT; i++) {
      const birdBaseIdx = CONFIG.SAB_OFFSET / 4 + i * (CONFIG.BYTES_PER_BIRD / 4);

      // --- Bird Base Transform ---
      // view indices: 0:x, 1:y, 2:z, 3:vx, 4:vy, 5:vz, 6:yaw, 7:pitch/bank, ...
      dummy.position.set(view[birdBaseIdx], view[birdBaseIdx + 1], view[birdBaseIdx + 2]);
      dummy.rotation.set(0, view[birdBaseIdx + 6], view[birdBaseIdx + 7]);
      dummy.updateMatrix();
      const birdMatrix = dummy.matrix;

      // 1. Body
      partDummy.position.set(0, 0, 0);
      partDummy.rotation.set(Math.PI / 2, 0, 0);
      partDummy.updateMatrix();
      bodiesRef.current?.setMatrixAt(i, matrix.multiplyMatrices(birdMatrix, partDummy.matrix));

      // 2. Head
      partDummy.position.set(0, 0, 0.18);
      partDummy.rotation.set(0, 0, 0);
      partDummy.updateMatrix();
      headsRef.current?.setMatrixAt(i, matrix.multiplyMatrices(birdMatrix, partDummy.matrix));

      // 3. Beak
      partDummy.position.set(0, 0, 0.26);
      partDummy.rotation.set(Math.PI / 2, 0, 0);
      partDummy.updateMatrix();
      beaksRef.current?.setMatrixAt(i, matrix.multiplyMatrices(birdMatrix, partDummy.matrix));

      // 4. Wings & Tips
      const flap = view[birdBaseIdx + 11];

      // Left Wing
      partDummy.position.set(-0.04, 0, 0.05);
      partDummy.rotation.set(0, 0, flap);
      partDummy.updateMatrix();
      m2.multiplyMatrices(birdMatrix, partDummy.matrix);

      partDummy.position.set(-0.15, 0, 0);
      partDummy.rotation.set(0, 0, 0);
      partDummy.updateMatrix();
      leftWingRef.current?.setMatrixAt(i, matrix.multiplyMatrices(m2, partDummy.matrix));

      partDummy.position.set(-0.3, 0, 0);
      partDummy.rotation.set(0, 0, flap * 0.5);
      partDummy.updateMatrix();
      m3.multiplyMatrices(m2, partDummy.matrix);

      partDummy.position.set(-0.12, 0, -0.05);
      partDummy.rotation.set(0, 0, 0);
      partDummy.updateMatrix();
      leftWingTipRef.current?.setMatrixAt(i, matrix.multiplyMatrices(m3, partDummy.matrix));

      // Right Wing
      partDummy.position.set(0.04, 0, 0.05);
      partDummy.rotation.set(0, 0, -flap);
      partDummy.updateMatrix();
      m2.multiplyMatrices(birdMatrix, partDummy.matrix);

      partDummy.position.set(0.15, 0, 0);
      partDummy.rotation.set(0, 0, 0);
      partDummy.updateMatrix();
      rightWingRef.current?.setMatrixAt(i, matrix.multiplyMatrices(m2, partDummy.matrix));

      partDummy.position.set(0.3, 0, 0);
      partDummy.rotation.set(0, 0, -flap * 0.5);
      partDummy.updateMatrix();
      m3.multiplyMatrices(m2, partDummy.matrix);

      partDummy.position.set(0.12, 0, -0.05);
      partDummy.rotation.set(0, 0, 0);
      partDummy.updateMatrix();
      rightWingTipRef.current?.setMatrixAt(i, matrix.multiplyMatrices(m3, partDummy.matrix));

      // 5. Tail
      partDummy.position.set(0, 0, -0.15);
      partDummy.rotation.set(0, view[birdBaseIdx + 13], 0);
      partDummy.updateMatrix();
      m2.multiplyMatrices(birdMatrix, partDummy.matrix);

      partDummy.position.set(0, 0, -0.1);
      partDummy.rotation.set(0, 0, 0);
      partDummy.updateMatrix();
      tailsRef.current?.setMatrixAt(i, matrix.multiplyMatrices(m2, partDummy.matrix));
    }

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
    instances.forEach(ref => {
      if (ref.current) ref.current.instanceMatrix.needsUpdate = true;
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
