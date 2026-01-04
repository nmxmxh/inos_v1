import { Canvas, useFrame, useThree } from '@react-three/fiber';
import { useRef, useMemo, Suspense, useEffect, useState } from 'react';
import * as THREE from 'three';
import { useSystemStore } from '../../src/store/system';

const CONFIG = {
  BIRD_COUNT: 1500,
  SAB_OFFSET: 0x400000,
  BYTES_PER_BIRD: 232,
};

// Debug Tracker - updates parent stats
function StatsTracker({ onUpdate }: { onUpdate: (stats: any) => void }) {
  const { gl } = useThree();
  useFrame(() => {
    onUpdate({
      draws: gl.info.render.calls,
      tris: gl.info.render.triangles,
      geoms: gl.info.memory.geometries,
    });
  });
  return null;
}

// Debug HUD - DOM side (Premium Light Theme for White Backgrounds)
function DebugHUD({ title, color, stats }: { title: string; color: string; stats: any }) {
  return (
    <div
      style={{
        position: 'absolute',
        padding: '12px',
        background: 'rgba(255, 255, 255, 0.75)',
        backdropFilter: 'blur(12px)',
        WebkitBackdropFilter: 'blur(12px)',
        color: '#1a1a1a',
        fontFamily: 'monospace',
        fontSize: '11px',
        borderRadius: '2px',
        border: '1px solid rgba(0, 0, 0, 0.08)',
        borderLeft: `3px solid ${color}`,
        boxShadow: '0 4px 15px rgba(0,0,0,0.05)',
        pointerEvents: 'none',
        zIndex: 100,
        width: '210px',
        top: '20px',
        left: '20px',
        letterSpacing: '0.02em',
      }}
    >
      <div
        style={{
          fontWeight: 'bold',
          marginBottom: '8px',
          fontSize: '9px',
          textTransform: 'uppercase',
          color: '#888',
          display: 'flex',
          justifyContent: 'space-between',
        }}
      >
        <span>{title}</span>
        <span style={{ opacity: 0.4 }}>INOS-CORE</span>
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr auto', gap: '4px' }}>
        <span style={{ color: '#666' }}>Draw Calls:</span>
        <span style={{ fontWeight: 'bold' }}>{stats.draws}</span>
        <span style={{ color: '#666' }}>Triangles:</span>
        <span style={{ fontWeight: 'bold' }}>{stats.tris.toLocaleString()}</span>
        <span style={{ color: '#666' }}>Geometries:</span>
        <span style={{ fontWeight: 'bold' }}>{stats.geoms}</span>
      </div>
      <div
        style={{
          marginTop: '10px',
          paddingTop: '8px',
          borderTop: '1px solid rgba(0,0,0,0.05)',
          fontSize: '8px',
          color: color,
          fontWeight: 'bold',
          letterSpacing: '0.05em',
        }}
      >
        ZERO-COPY VERTEX ENGINE
      </div>
    </div>
  );
}

function InstancedBoidsRenderer({ onStatsUpdate }: { onStatsUpdate: (stats: any) => void }) {
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
      body: new THREE.CylinderGeometry(0.02, 0.04, 0.3, 8),
      head: new THREE.BoxGeometry(0.08, 0.08, 0.08),
      beak: new THREE.ConeGeometry(0.02, 0.15, 6),
      wing: new THREE.PlaneGeometry(0.3, 0.12),
      wingTip: new THREE.PlaneGeometry(0.25, 0.08),
      tail: new THREE.PlaneGeometry(0.05, 0.2),
    }),
    []
  );

  // Shared dummy for math
  const dummy = useMemo(() => new THREE.Object3D(), []);
  const partDummy = useMemo(() => new THREE.Object3D(), []);
  const matrix = useMemo(() => new THREE.Matrix4(), []);

  // Initialize colors
  const colors = useMemo(() => {
    const palette = ['#6d28d9', '#ec4899', '#10b981', '#f59e0b'];
    return Array.from(
      { length: CONFIG.BIRD_COUNT },
      (_, i) => new THREE.Color(palette[i % palette.length])
    );
  }, []);

  // Initialize ONCE - use ref to prevent re-init on moduleExports changes
  useEffect(() => {
    const exports = moduleExportsRef.current;
    if (exports.compute?.compute_boids_init) {
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
          const color = colors[i];
          if (ref === leftWingRef || ref === rightWingRef) {
            ref.current.setColorAt(i, new THREE.Color('#555555'));
          } else {
            ref.current.setColorAt(i, color);
          }
        }
        ref.current.instanceColor!.needsUpdate = true;
      }
    });
  }, [colors]);

  useFrame((_, delta) => {
    // 1. Run physics step in WASM
    if (moduleExports.compute?.compute_boids_step) {
      moduleExports.compute.compute_boids_step(CONFIG.BIRD_COUNT, delta);
    }

    // 2. Read state from SAB and update matrices
    const sab = (window as any).__INOS_SAB__;
    if (!sab) return;

    for (let i = 0; i < CONFIG.BIRD_COUNT; i++) {
      const base = CONFIG.SAB_OFFSET + i * CONFIG.BYTES_PER_BIRD;
      const view = new Float32Array(sab, base, 58);

      // --- Bird Base Transform ---
      dummy.position.set(view[0], view[1], view[2]);
      dummy.rotation.set(0, view[6], view[7]);
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
      const flap = view[11];

      // Left Wing
      partDummy.position.set(-0.04, 0, 0.05);
      partDummy.rotation.set(0, 0, flap);
      partDummy.updateMatrix();
      const leftBaseMatrix = new THREE.Matrix4().multiplyMatrices(birdMatrix, partDummy.matrix);

      partDummy.position.set(-0.15, 0, 0);
      partDummy.rotation.set(0, 0, 0);
      partDummy.updateMatrix();
      leftWingRef.current?.setMatrixAt(
        i,
        matrix.multiplyMatrices(leftBaseMatrix, partDummy.matrix)
      );

      partDummy.position.set(-0.3, 0, 0);
      partDummy.rotation.set(0, 0, flap * 0.5);
      partDummy.updateMatrix();
      const leftTipParentMatrix = new THREE.Matrix4().multiplyMatrices(
        leftBaseMatrix,
        partDummy.matrix
      );
      partDummy.position.set(-0.12, 0, -0.05);
      partDummy.rotation.set(0, 0, 0);
      partDummy.updateMatrix();
      leftWingTipRef.current?.setMatrixAt(
        i,
        matrix.multiplyMatrices(leftTipParentMatrix, partDummy.matrix)
      );

      // Right Wing
      partDummy.position.set(0.04, 0, 0.05);
      partDummy.rotation.set(0, 0, -flap);
      partDummy.updateMatrix();
      const rightBaseMatrix = new THREE.Matrix4().multiplyMatrices(birdMatrix, partDummy.matrix);

      partDummy.position.set(0.15, 0, 0);
      partDummy.rotation.set(0, 0, 0);
      partDummy.updateMatrix();
      rightWingRef.current?.setMatrixAt(
        i,
        matrix.multiplyMatrices(rightBaseMatrix, partDummy.matrix)
      );

      partDummy.position.set(0.3, 0, 0);
      partDummy.rotation.set(0, 0, -flap * 0.5);
      partDummy.updateMatrix();
      const rightTipParentMatrix = new THREE.Matrix4().multiplyMatrices(
        rightBaseMatrix,
        partDummy.matrix
      );
      partDummy.position.set(0.12, 0, -0.05);
      partDummy.rotation.set(0, 0, 0);
      partDummy.updateMatrix();
      rightWingTipRef.current?.setMatrixAt(
        i,
        matrix.multiplyMatrices(rightTipParentMatrix, partDummy.matrix)
      );

      // 5. Tail
      partDummy.position.set(0, 0, -0.15);
      partDummy.rotation.set(0, view[13], 0);
      partDummy.updateMatrix();
      const tailMatrix = new THREE.Matrix4().multiplyMatrices(birdMatrix, partDummy.matrix);
      partDummy.position.set(0, 0, -0.1);
      partDummy.rotation.set(0, 0, 0);
      partDummy.updateMatrix();
      tailsRef.current?.setMatrixAt(i, matrix.multiplyMatrices(tailMatrix, partDummy.matrix));
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
      <StatsTracker onUpdate={onStatsUpdate} />
      <instancedMesh ref={bodiesRef} args={[geometries.body, undefined, CONFIG.BIRD_COUNT]}>
        <meshBasicMaterial />
      </instancedMesh>
      <instancedMesh ref={headsRef} args={[geometries.head, undefined, CONFIG.BIRD_COUNT]}>
        <meshBasicMaterial />
      </instancedMesh>
      <instancedMesh ref={beaksRef} args={[geometries.beak, undefined, CONFIG.BIRD_COUNT]}>
        <meshBasicMaterial />
      </instancedMesh>
      <instancedMesh ref={leftWingRef} args={[geometries.wing, undefined, CONFIG.BIRD_COUNT]}>
        <meshBasicMaterial side={THREE.DoubleSide} transparent opacity={0.4} />
      </instancedMesh>
      <instancedMesh ref={leftWingTipRef} args={[geometries.wingTip, undefined, CONFIG.BIRD_COUNT]}>
        <meshBasicMaterial side={THREE.DoubleSide} transparent opacity={0.2} />
      </instancedMesh>
      <instancedMesh ref={rightWingRef} args={[geometries.wing, undefined, CONFIG.BIRD_COUNT]}>
        <meshBasicMaterial side={THREE.DoubleSide} transparent opacity={0.4} />
      </instancedMesh>
      <instancedMesh
        ref={rightWingTipRef}
        args={[geometries.wingTip, undefined, CONFIG.BIRD_COUNT]}
      >
        <meshBasicMaterial side={THREE.DoubleSide} transparent opacity={0.2} />
      </instancedMesh>
      <instancedMesh ref={tailsRef} args={[geometries.tail, undefined, CONFIG.BIRD_COUNT]}>
        <meshBasicMaterial side={THREE.DoubleSide} transparent opacity={0.5} />
      </instancedMesh>
    </>
  );
}

export default function ArchitecturalBoids() {
  const [stats, setStats] = useState({ draws: 0, tris: 0, geoms: 0 });

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
      <DebugHUD title="INOS ZERO-COPY ENGINE" color="#6d28d9" stats={stats} />
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
          <InstancedBoidsRenderer onStatsUpdate={setStats} />
          <gridHelper args={[80, 10, '#330066', '#110022']} position={[0, -10, 0]} />
        </Canvas>
      </Suspense>
    </div>
  );
}
