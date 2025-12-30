import { useRef, useMemo } from 'react';
import { Canvas, useFrame } from '@react-three/fiber';
import { PerspectiveCamera, Stars, Float } from '@react-three/drei';
import * as THREE from 'three';

const BirdBody = () => {
  const group = useRef<THREE.Group>(null);
  const leftWing = useRef<THREE.Mesh>(null);
  const rightWing = useRef<THREE.Mesh>(null);

  // Performance optimized geometry
  const bodyGeo = useMemo(() => new THREE.ConeGeometry(0.2, 1, 4), []);
  const wingGeo = useMemo(() => new THREE.PlaneGeometry(0.5, 1, 1), []);

  useFrame(state => {
    if (!group.current || !leftWing.current || !rightWing.current) return;

    const t = state.clock.getElapsedTime();

    // 1. Science Layer: Kinematic Flapping
    const flapSpeed = 8;
    const flapStrength = 0.5;
    const flapAngle = Math.sin(t * flapSpeed) * flapStrength;

    leftWing.current.rotation.y = -flapAngle - 0.5;
    rightWing.current.rotation.y = flapAngle + 0.5;

    // 2. ML Layer: Behavioral Pathfinding (Lissajous-based Search)
    group.current.position.y = Math.sin(t * 0.5) * 2;
    group.current.position.x = Math.sin(t * 0.3) * 5;
    group.current.position.z = Math.cos(t * 0.4) * 3;

    // Look ahead
    const targetPos = new THREE.Vector3(
      Math.sin((t + 0.1) * 0.3) * 5,
      Math.sin((t + 0.1) * 0.5) * 2,
      Math.cos((t + 0.1) * 0.4) * 3
    );
    group.current.lookAt(targetPos);
  });

  return (
    <group ref={group}>
      {/* Fuselage / Body */}
      <mesh geometry={bodyGeo} rotation={[Math.PI / 2, 0, 0]}>
        <meshStandardMaterial color="#3b82f6" emissive="#3b82f6" emissiveIntensity={2} wireframe />
      </mesh>

      {/* Left Wing */}
      <mesh ref={leftWing} geometry={wingGeo} position={[-0.3, 0, 0]} rotation={[0, 0, 0]}>
        <meshStandardMaterial
          color="#60a5fa"
          emissive="#60a5fa"
          emissiveIntensity={1}
          wireframe
          side={THREE.DoubleSide}
        />
      </mesh>

      {/* Right Wing */}
      <mesh ref={rightWing} geometry={wingGeo} position={[0.3, 0, 0]} rotation={[0, 0, 0]}>
        <meshStandardMaterial
          color="#60a5fa"
          emissive="#60a5fa"
          emissiveIntensity={1}
          wireframe
          side={THREE.DoubleSide}
        />
      </mesh>

      {/* Core Unit (Central Glow) */}
      <mesh>
        <sphereGeometry args={[0.08, 16, 16]} />
        <meshStandardMaterial color="#fff" emissive="#ffd700" emissiveIntensity={5} />
      </mesh>
    </group>
  );
};

const BigBangParticles = () => {
  const count = 1000;
  const points = useRef<THREE.Points>(null);

  const particles = useMemo(() => {
    const pos = new Float32Array(count * 3);
    for (let i = 0; i < count; i++) {
      pos[i * 3 + 0] = (Math.random() - 0.5) * 20;
      pos[i * 3 + 1] = (Math.random() - 0.5) * 20;
      pos[i * 3 + 2] = (Math.random() - 0.5) * 20;
    }
    return pos;
  }, []);

  useFrame(state => {
    if (!points.current) return;
    const t = state.clock.getElapsedTime();
    // Simple Big Bang expansion simulation
    points.current.rotation.y = t * 0.05;
    points.current.scale.setScalar(1 + Math.sin(t * 0.1) * 0.1);
  });

  return (
    <points ref={points}>
      <bufferGeometry>
        <bufferAttribute
          attach="attributes-position"
          count={count}
          array={particles}
          itemSize={3}
        />
      </bufferGeometry>
      <pointsMaterial size={0.05} color="#fff" transparent opacity={0.6} sizeAttenuation />
    </points>
  );
};

export default function BirdCanvas() {
  return (
    <div
      style={{
        width: '100%',
        height: '400px',
        background: 'radial-gradient(circle at center, #0a0a1a 0%, #000 100%)',
        borderRadius: '16px',
        overflow: 'hidden',
        border: '1px solid rgba(255,255,255,0.05)',
        position: 'relative',
      }}
    >
      {/* Overlay Labels */}
      <div
        style={{
          position: 'absolute',
          top: '20px',
          left: '20px',
          zIndex: 1,
          pointerEvents: 'none',
        }}
      >
        <div
          style={{ fontSize: '10px', color: '#667eea', fontWeight: 'bold', letterSpacing: '2px' }}
        >
          UNIT SIMULATION
        </div>
        <div style={{ fontSize: '18px', color: '#fff', fontWeight: 'bold' }}>
          Cybernetic Kinematics
        </div>
      </div>

      <div
        style={{
          position: 'absolute',
          bottom: '20px',
          right: '20px',
          zIndex: 1,
          pointerEvents: 'none',
          textAlign: 'right',
        }}
      >
        <div style={{ fontSize: '10px', color: '#ff4444' }}>ENGINE: SCIENCE::KINETIC</div>
        <div style={{ fontSize: '10px', color: '#3b82f6' }}>LOGIC: ML::INFERENCE</div>
      </div>

      <Canvas shadows gl={{ antialias: true }}>
        <PerspectiveCamera makeDefault position={[0, 0, 10]} />
        <ambientLight intensity={0.2} />
        <pointLight position={[10, 10, 10]} intensity={1} color="#3b82f6" />
        <pointLight position={[-10, -10, -10]} intensity={0.5} color="#ff00ff" />

        <Stars radius={100} depth={50} count={5000} factor={4} saturation={0} fade speed={1} />

        <BigBangParticles />

        <Float speed={2} rotationIntensity={0.5} floatIntensity={0.5}>
          <BirdBody />
        </Float>
      </Canvas>
    </div>
  );
}
