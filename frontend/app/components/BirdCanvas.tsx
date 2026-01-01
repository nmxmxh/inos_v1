import { useRef, useMemo } from 'react';
import { Canvas, useFrame } from '@react-three/fiber';
import { PerspectiveCamera, Stars, Float } from '@react-three/drei';
import * as THREE from 'three';

const BIRD_STATE_OFFSET = 0x160000;

const BirdBody = () => {
  const group = useRef<THREE.Group>(null);
  const leftWing = useRef<THREE.Mesh>(null);
  const rightWing = useRef<THREE.Mesh>(null);

  // Performance optimized geometry
  const bodyGeo = useMemo(() => new THREE.ConeGeometry(0.2, 1, 4), []);
  const wingGeo = useMemo(() => new THREE.PlaneGeometry(0.5, 1, 1), []);

  const mouseRef = useRef(new THREE.Vector2());

  // Handle interaction point mutation
  const handlePointerMove = (e: any) => {
    mouseRef.current.set(e.point.x, e.point.y);
  };

  // Cybernetic Shader Material
  const material = useMemo(
    () =>
      new THREE.ShaderMaterial({
        uniforms: {
          uTime: { value: 0 },
          uColor: { value: new THREE.Color('#3b82f6') },
          uOpacity: { value: 0.8 },
          uGlow: { value: 2.0 },
        },
        transparent: true,
        side: THREE.DoubleSide,
        vertexShader: `
        varying vec2 vUv;
        varying vec3 vNormal;
        varying vec3 vViewPosition;
        void main() {
          vUv = uv;
          vNormal = normalize(normalMatrix * normal);
          vec4 mvPosition = modelViewMatrix * vec4(position, 1.0);
          vViewPosition = -mvPosition.xyz;
          gl_Position = projectionMatrix * mvPosition;
        }
      `,
        fragmentShader: `
        varying vec2 vUv;
        varying vec3 vNormal;
        varying vec3 vViewPosition;
        uniform float uTime;
        uniform vec3 uColor;
        uniform float uOpacity;
        uniform float uGlow;

        void main() {
          vec3 normal = normalize(vNormal);
          vec3 viewDir = normalize(vViewPosition);
          float fresnel = pow(1.0 - dot(normal, viewDir), 3.0);
          
          // Scanlines
          float scanline = sin(vUv.y * 100.0 + uTime * 10.0) * 0.1 + 0.9;
          
          // Glitch pulse
          float pulse = sin(uTime * 5.0) * 0.1 + 0.9;
          
          vec3 finalColor = uColor * (fresnel * uGlow + 0.2) * scanline * pulse;
          gl_FragColor = vec4(finalColor, uOpacity * (fresnel + 0.1));
        }
      `,
      }),
    []
  );

  useFrame(state => {
    if (!group.current || !leftWing.current || !rightWing.current) return;

    const sab = (window as any).__INOS_SAB__;
    if (!sab) return;

    // Zero-Copy Read: Access SAB directly via Float32Array overlay
    // struct BirdState { position[3], velocity[3], orientation[4], flap_phase, energy, interaction_point[3], wing_flap_speed }
    const birdState = new Float32Array(sab, BIRD_STATE_OFFSET, 32);
    const pos = { x: birdState[0], y: birdState[1], z: birdState[2] };
    const flapPhase = birdState[10];

    // 1. Position from Rust
    group.current.position.set(pos.x, pos.y, pos.z);

    // 2. Kinematic Flapping (Semi-driven by Rust flapPhase)
    const flapAngle = Math.sin(flapPhase * Math.PI * 2) * 0.6;
    leftWing.current.rotation.y = -flapAngle - 0.5;
    rightWing.current.rotation.y = flapAngle + 0.5;

    // 3. Look ahead (Interpolated)
    const vel = new THREE.Vector3(birdState[3], birdState[4], birdState[5]);
    if (vel.length() > 0.01) {
      const target = group.current.position.clone().add(vel);
      group.current.lookAt(target);
    }

    // 4. Concurrent Mutation: JS Jitter
    const jitter = Math.sin(state.clock.getElapsedTime() * 100) * 0.005;
    group.current.position.addScalar(jitter);

    // Update shader
    material.uniforms.uTime.value = state.clock.getElapsedTime();
  });

  return (
    <group ref={group} onPointerMove={handlePointerMove}>
      {/* Fuselage / Body */}
      <mesh geometry={bodyGeo} rotation={[Math.PI / 2, 0, 0]} material={material} />

      {/* Wings */}
      <mesh ref={leftWing} geometry={wingGeo} position={[-0.3, 0, 0]} material={material} />
      <mesh ref={rightWing} geometry={wingGeo} position={[0.3, 0, 0]} material={material} />

      {/* Core Unit (Central Glow) */}
      <mesh>
        <sphereGeometry args={[0.08, 16, 16]} />
        <meshStandardMaterial color="#fff" emissive="#ffd700" emissiveIntensity={5} />
      </mesh>
    </group>
  );
};

const BigBangParticles = () => {
  const count = 1500;
  const points = useRef<THREE.Points>(null);
  const IDX_OUTBOX_DIRTY = 2; // Signal from Module to Kernel

  const particles = useMemo(() => {
    const pos = new Float32Array(count * 3);
    const vel = new Float32Array(count * 3);
    for (let i = 0; i < count; i++) {
      pos[i * 3 + 0] = (Math.random() - 0.5) * 20;
      pos[i * 3 + 1] = (Math.random() - 0.5) * 20;
      pos[i * 3 + 2] = (Math.random() - 0.5) * 20;
      vel[i * 3 + 0] = (Math.random() - 0.5) * 0.05;
      vel[i * 3 + 1] = (Math.random() - 0.5) * 0.05;
      vel[i * 3 + 2] = (Math.random() - 0.5) * 0.05;
    }
    return { pos, vel };
  }, []);

  const lastEpoch = useRef(0);

  useFrame(() => {
    if (!points.current) return;
    const sab = (window as any).__INOS_SAB__;
    if (!sab) return;

    const flags = new Int32Array(sab, 0, 16);
    const currentEpoch = Atomics.load(flags, IDX_OUTBOX_DIRTY);

    // Data Burst Effect: Triggered when Go Kernel gossips mesh state
    const isBursting = currentEpoch !== lastEpoch.current;
    if (isBursting) lastEpoch.current = currentEpoch;

    const pos = points.current.geometry.attributes.position.array as Float32Array;
    for (let i = 0; i < count; i++) {
      const idx = i * 3;
      // Normal drift
      pos[idx] += particles.vel[idx];
      pos[idx + 1] += particles.vel[idx + 1];
      pos[idx + 2] += particles.vel[idx + 2];

      // Reset if too far
      if (Math.abs(pos[idx]) > 15) pos[idx] *= -0.8;
      if (Math.abs(pos[idx + 1]) > 15) pos[idx + 1] *= -0.8;

      // Burst expansion
      if (isBursting) {
        pos[idx] *= 1.1;
        pos[idx + 1] *= 1.1;
        pos[idx + 2] *= 1.1;
      }
    }
    points.current.geometry.attributes.position.array.set(particles.pos);
    points.current.geometry.attributes.position.needsUpdate = true;
    points.current.rotation.y += 0.001;
  });

  return (
    <points ref={points}>
      <bufferGeometry>
        <bufferAttribute
          attach="attributes-position"
          count={count}
          array={particles.pos}
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
        <PerspectiveCamera makeDefault position={[0, 0, 12]} />
        <ambientLight intensity={0.5} />
        <pointLight position={[10, 10, 10]} intensity={2} color="#3b82f6" />
        <pointLight position={[-10, -10, -10]} intensity={1} color="#ff00ff" />

        <Stars radius={100} depth={50} count={5000} factor={4} saturation={0} fade speed={1} />

        <BigBangParticles />

        <Float speed={3} rotationIntensity={0.2} floatIntensity={0.2}>
          <BirdBody />
        </Float>

        {/* Interaction Plane */}
        <mesh
          visible={false}
          onPointerMove={e => {
            const sab = (window as any).__INOS_SAB__;
            if (sab) {
              const birdState = new Float32Array(sab, BIRD_STATE_OFFSET, 32);
              // Indices 12, 13, 14: interaction_point
              birdState[12] = e.point.x;
              birdState[13] = e.point.y;
              birdState[14] = e.point.z;
            }
          }}
        >
          <planeGeometry args={[50, 50]} />
        </mesh>
      </Canvas>
    </div>
  );
}
