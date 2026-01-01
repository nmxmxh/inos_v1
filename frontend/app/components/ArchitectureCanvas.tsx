import { useRef, useMemo, useState, useEffect } from 'react';
import { Canvas, useFrame } from '@react-three/fiber';
import { PerspectiveCamera, Stars, Sparkles } from '@react-three/drei';
import * as THREE from 'three';
import { useEnhancedModuleOrchestrator } from '../../src/hooks/useEnhancedModuleOrchestrator';

const BIRD_STATE_OFFSET = 0x160000;
const NODE_COUNT = 8;

// Enhanced compute node with GPU-accelerated effects
const ComputeNode = ({ index, total }: { index: number; total: number }) => {
  const mesh = useRef<THREE.Mesh>(null);
  const glow = useRef<THREE.Mesh>(null);

  const orbit = useMemo(() => {
    const angle = (index / total) * Math.PI * 2;
    const radius = 6 + Math.sin(angle * 3) * 2;
    const speed = 0.3 + (index % 3) * 0.1;
    const phase = angle;
    return { radius, speed, phase, angle };
  }, [index, total]);

  const color = useMemo(() => {
    const colors = ['#3b82f6', '#8b5cf6', '#ec4899', '#f59e0b', '#10b981', '#06b6d4'];
    return new THREE.Color(colors[index % colors.length]);
  }, [index]);

  useFrame(state => {
    if (!mesh.current || !glow.current) return;

    const t = state.clock.getElapsedTime() * orbit.speed + orbit.phase;
    const x = Math.cos(t) * orbit.radius;
    const y = Math.sin(t * 0.7) * 2;
    const z = Math.sin(t) * orbit.radius;

    mesh.current.position.set(x, y, z);
    glow.current.position.copy(mesh.current.position);

    // Pulsing effect
    const scale = 1 + Math.sin(t * 4) * 0.2;
    mesh.current.scale.setScalar(scale);
    glow.current.scale.setScalar(scale * 1.5);
  });

  return (
    <group>
      <mesh ref={mesh}>
        <icosahedronGeometry args={[0.3, 1]} />
        <meshStandardMaterial
          color={color}
          emissive={color}
          emissiveIntensity={2}
          metalness={0.8}
          roughness={0.2}
        />
      </mesh>

      {/* Glow effect */}
      <mesh ref={glow}>
        <sphereGeometry args={[0.5, 16, 16]} />
        <meshBasicMaterial color={color} transparent opacity={0.2} />
      </mesh>
    </group>
  );
};

// GPU-accelerated N-body particle system with WASM compute shader simulation
const DataFlowParticles = () => {
  const points = useRef<THREE.Points>(null);
  const count = 150; // Enhanced particles with full physics
  const lastEpoch = useRef(0);
  const initialized = useRef(false);

  // Initialize enhanced N-body simulation
  const orchestrator = useEnhancedModuleOrchestrator(count, {
    forceLaw: 0, // Newtonian
    enableCollisions: true,
    turbulenceStrength: 0.05,
    magneticStrength: 0.02,
  });

  const particles = useMemo(() => {
    const pos = new Float32Array(count * 3);
    const colors = new Float32Array(count * 3);
    const sizes = new Float32Array(count);

    // Initialize with galaxy-like distribution
    for (let i = 0; i < count; i++) {
      const theta = Math.random() * Math.PI * 2;
      const phi = Math.random() * Math.PI;
      const r = 5 + Math.random() * 8;

      pos[i * 3] = r * Math.sin(phi) * Math.cos(theta);
      pos[i * 3 + 1] = r * Math.sin(phi) * Math.sin(theta) * 0.3; // Flattened disk
      pos[i * 3 + 2] = r * Math.cos(phi);

      // Initial colors (will be updated by physics)
      const color = new THREE.Color().setHSL(Math.random(), 0.8, 0.6);
      colors[i * 3] = color.r;
      colors[i * 3 + 1] = color.g;
      colors[i * 3 + 2] = color.b;

      sizes[i] = 0.1 + Math.random() * 0.2;
    }

    return { pos, colors, sizes };
  }, []);

  // Initialize WASM simulation
  useEffect(() => {
    if (initialized.current) return;

    const initPositions: number[] = [];
    const initVelocities: number[] = [];
    const masses: number[] = [];
    const types: number[] = [];

    for (let i = 0; i < count; i++) {
      // Position
      initPositions.push(particles.pos[i * 3], particles.pos[i * 3 + 1], particles.pos[i * 3 + 2]);

      // Orbital velocity for disk
      const x = particles.pos[i * 3];
      const z = particles.pos[i * 3 + 2];
      const r = Math.sqrt(x * x + z * z);
      const orbitalSpeed = r > 0 ? Math.sqrt((5.0 * 100) / r) * 0.1 : 0;
      initVelocities.push((-z * orbitalSpeed) / r, 0, (x * orbitalSpeed) / r);

      // Mass (varied)
      masses.push(1.5 + Math.random() * 2);

      // Particle types: mix of normal (0), stars (1), black holes (2), dark matter (3)
      const rand = Math.random();
      if (rand < 0.6)
        types.push(0); // 60% normal
      else if (rand < 0.85)
        types.push(1); // 25% stars
      else if (rand < 0.95)
        types.push(3); // 10% dark matter
      else types.push(2); // 5% black holes
    }

    orchestrator.initializeParticles(initPositions, initVelocities, masses, types);
    initialized.current = true;
    console.log('[ArchitectureCanvas] Enhanced N-body initialized with WASM compute');
  }, [orchestrator, particles.pos]);

  useFrame(state => {
    if (!points.current) return;

    const sab = (window as any).__INOS_SAB__;
    const flags = sab ? new Int32Array(sab, 0, 16) : null;
    const epoch = flags ? Atomics.load(flags, 7) : 0; // System epoch

    // Detect epoch change for burst effect
    const isBurst = epoch !== lastEpoch.current && epoch > 0;
    if (isBurst) lastEpoch.current = epoch;

    const pos = points.current.geometry.attributes.position.array as Float32Array;
    const colors = points.current.geometry.attributes.color.array as Float32Array;
    const sizes = points.current.geometry.attributes.size.array as Float32Array;

    // Update from WASM compute
    if (initialized.current) {
      // Step physics
      orchestrator.step(0.016);

      // Get enhanced particle data
      const enhancedParticles = orchestrator.getParticles();

      for (let i = 0; i < count && i < enhancedParticles.length; i++) {
        const p = enhancedParticles[i];
        const idx = i * 3;

        // Update position from physics simulation
        pos[idx] = p.position[0] * 0.1; // Scale for 3D view
        pos[idx + 1] = p.position[1] * 0.1;
        pos[idx + 2] = p.position[2] * 0.1;

        // Update color from particle properties
        const color = new THREE.Color(p.color[0], p.color[1], p.color[2]);

        // Burst effect on epoch change
        if (isBurst) {
          colors[idx] = 1;
          colors[idx + 1] = 1;
          colors[idx + 2] = 1;
        } else {
          // Use physics-based colors
          colors[idx] = color.r;
          colors[idx + 1] = color.g;
          colors[idx + 2] = color.b;
        }

        // Size based on radius and luminosity (bloom effect)
        const bloom = p.luminosity > 0.5 ? Math.sqrt(p.luminosity) * 0.5 : 0;
        sizes[i] = p.radius * 0.02 * (1 + bloom);

        // Type-specific effects
        if (p.particleType === 2) {
          // Black hole
          sizes[i] *= 1.5;
          colors[idx] *= 0.3;
          colors[idx + 1] *= 0.3;
          colors[idx + 2] *= 0.5;
        } else if (p.particleType === 3) {
          // Dark matter
          colors[idx] *= 0.5;
          colors[idx + 1] *= 0.3;
          colors[idx + 2] *= 0.7;
        }
      }
    } else {
      // Fallback: simple motion if not initialized yet
      const t = state.clock.getElapsedTime();
      for (let i = 0; i < count; i++) {
        const idx = i * 3;
        const dx = -pos[idx] * 0.001;
        const dy = -pos[idx + 1] * 0.001;
        const dz = -pos[idx + 2] * 0.001;
        pos[idx] += dx;
        pos[idx + 1] += dy;
        pos[idx + 2] += dz;

        const color = new THREE.Color().setHSL((i / count + t * 0.1) % 1, 0.8, 0.6);
        colors[idx] = color.r;
        colors[idx + 1] = color.g;
        colors[idx + 2] = color.b;
      }
    }

    points.current.geometry.attributes.position.needsUpdate = true;
    points.current.geometry.attributes.color.needsUpdate = true;
    points.current.geometry.attributes.size.needsUpdate = true;
    points.current.rotation.y += 0.0005;
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
        <bufferAttribute
          attach="attributes-color"
          count={count}
          array={particles.colors}
          itemSize={3}
        />
        <bufferAttribute
          attach="attributes-size"
          count={count}
          array={particles.sizes}
          itemSize={1}
        />
      </bufferGeometry>
      <pointsMaterial
        size={0.15}
        vertexColors
        transparent
        opacity={0.8}
        sizeAttenuation
        blending={THREE.AdditiveBlending}
      />
    </points>
  );
};

// Central kernel with GPU-enhanced shaders
const KernelCore = () => {
  const core = useRef<THREE.Mesh>(null);
  const rings = useRef<THREE.Group>(null);

  // Custom shader material for core
  const coreMaterial = useMemo(
    () =>
      new THREE.ShaderMaterial({
        uniforms: {
          uTime: { value: 0 },
          uColor: { value: new THREE.Color('#ffd700') },
        },
        vertexShader: `
          varying vec3 vNormal;
          varying vec3 vPosition;
          void main() {
            vNormal = normalize(normalMatrix * normal);
            vPosition = position;
            gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
          }
        `,
        fragmentShader: `
          uniform float uTime;
          uniform vec3 uColor;
          varying vec3 vNormal;
          varying vec3 vPosition;
          
          void main() {
            // Fresnel effect
            vec3 viewDir = normalize(cameraPosition - vPosition);
            float fresnel = pow(1.0 - dot(vNormal, viewDir), 3.0);
            
            // Pulsing energy
            float pulse = sin(uTime * 2.0) * 0.5 + 0.5;
            
            // Scanlines
            float scanline = sin(vPosition.y * 20.0 + uTime * 5.0) * 0.1 + 0.9;
            
            vec3 finalColor = uColor * (1.0 + fresnel * 2.0) * pulse * scanline;
            gl_FragColor = vec4(finalColor, 1.0);
          }
        `,
      }),
    []
  );

  useFrame(state => {
    if (!core.current || !rings.current) return;

    const t = state.clock.getElapsedTime();
    core.current.rotation.x = t * 0.3;
    core.current.rotation.y = t * 0.5;

    rings.current.rotation.x = -t * 0.2;
    rings.current.rotation.z = t * 0.4;

    // Pulse
    const scale = 1 + Math.sin(t * 2) * 0.1;
    core.current.scale.setScalar(scale);

    // Update shader time
    coreMaterial.uniforms.uTime.value = t;
  });

  return (
    <group>
      {/* Core with custom shader */}
      <mesh ref={core} material={coreMaterial}>
        <octahedronGeometry args={[1, 2]} />
      </mesh>

      {/* Orbital rings */}
      <group ref={rings}>
        {[1.5, 2, 2.5].map((radius, i) => (
          <mesh key={i} rotation={[Math.PI / 2, 0, i * 0.5]}>
            <torusGeometry args={[radius, 0.02, 16, 100]} />
            <meshBasicMaterial color="#3b82f6" transparent opacity={0.3} />
          </mesh>
        ))}
      </group>

      <Sparkles count={50} scale={3} size={2} speed={0.3} color="#ffd700" />
    </group>
  );
};

// Main canvas component
export default function ArchitectureCanvas() {
  const [stats, setStats] = useState({
    fps: 0,
    nodes: NODE_COUNT,
    particles: 150, // Enhanced N-body particles
    epoch: 0,
  });

  useEffect(() => {
    const interval = setInterval(() => {
      const sab = (window as any).__INOS_SAB__;
      if (sab) {
        const flags = new Int32Array(sab, 0, 16);
        const epoch = Atomics.load(flags, 2);
        setStats(prev => ({ ...prev, epoch }));
      }
    }, 1000);
    return () => clearInterval(interval);
  }, []);

  return (
    <div
      style={{
        position: 'fixed',
        top: 0,
        left: 0,
        width: '100vw',
        height: '100vh',
        background: 'radial-gradient(ellipse at center, #0a0a1a 0%, #000000 100%)',
        zIndex: 0,
      }}
    >
      {/* Performance overlay */}
      <div
        style={{
          position: 'absolute',
          top: '20px',
          right: '20px',
          zIndex: 10,
          fontFamily: 'monospace',
          fontSize: '11px',
          color: '#666',
          textAlign: 'right',
          pointerEvents: 'none',
        }}
      >
        <div style={{ color: '#3b82f6', fontWeight: 'bold', marginBottom: '4px' }}>
          MESH TOPOLOGY
        </div>
        <div>
          NODES: <span style={{ color: '#fff' }}>{stats.nodes}</span>
        </div>
        <div>
          PARTICLES: <span style={{ color: '#fff' }}>{stats.particles}</span>
        </div>
        <div>
          EPOCH: <span style={{ color: '#10b981' }}>{stats.epoch || 0}</span>
        </div>
      </div>

      <Canvas shadows dpr={[1, 2]} gl={{ antialias: true, alpha: false }}>
        <PerspectiveCamera makeDefault position={[0, 3, 15]} fov={60} />

        {/* Lighting */}
        <ambientLight intensity={0.3} />
        <pointLight position={[10, 10, 10]} intensity={1} color="#3b82f6" />
        <pointLight position={[-10, -10, -10]} intensity={0.5} color="#ec4899" />
        <spotLight
          position={[0, 10, 0]}
          angle={0.3}
          penumbra={1}
          intensity={1}
          castShadow
          color="#ffffff"
        />

        {/* Background */}
        <Stars radius={100} depth={50} count={5000} factor={4} saturation={0} fade speed={1} />

        {/* Central kernel with GPU shader */}
        <KernelCore />

        {/* Compute nodes */}
        {Array.from({ length: NODE_COUNT }).map((_, i) => (
          <ComputeNode key={i} index={i} total={NODE_COUNT} />
        ))}

        {/* GPU-accelerated data flow */}
        <DataFlowParticles />

        {/* Interaction plane */}
        <mesh
          visible={false}
          onPointerMove={e => {
            const sab = (window as any).__INOS_SAB__;
            if (sab) {
              const birdState = new Float32Array(sab, BIRD_STATE_OFFSET, 32);
              birdState[12] = e.point.x;
              birdState[13] = e.point.y;
              birdState[14] = e.point.z;
            }
          }}
        >
          <planeGeometry args={[100, 100]} />
        </mesh>
      </Canvas>
    </div>
  );
}
