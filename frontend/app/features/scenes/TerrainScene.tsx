/**
 * INOS Graphics Scenes — Procedural Terrain with Water
 *
 * Architecture (per graphics.md):
 * - Zero-Copy SAB: Heightmap in shared memory
 * - GPU Unit: dispatch.execute('gpu', 'perlin_noise')
 * - Life-like terrain with animated water plane
 * - Serves as background for boids simulation
 */

import { useRef, useMemo, useEffect, useState } from 'react';
import { useFrame } from '@react-three/fiber';
import * as THREE from 'three';
import { Stars, Float } from '@react-three/drei';
import SceneWrapper, { useSAB, SCENE_OFFSETS, getArenaView } from './SceneWrapper';
import { useSystemStore } from '../../../src/store/system';
import { dispatch } from '../../../src/wasm/dispatch';
import InstancedBoidsRenderer from '../boids/InstancedBoidsRenderer';

// ========== CONFIGURATION ==========

const CONFIG = {
  GRID_SIZE: 100,
  SCALE: 60,
  HEIGHT_SCALE: 12,
  WATER_LEVEL: -5.0,
  OCTAVES: 6,
  PERSISTENCE: 0.45,
  ANIMATION_SPEED: 0.02,
  CLOUD_GRID: 64,
  CLOUD_SCALE: 120,
  CLOUD_HEIGHT: 25,
};

// Biome color palette - realistic terrain
const BIOME = {
  deepWater: new THREE.Color('#0c4a6e'),
  water: new THREE.Color('#0284c7'),
  shallowWater: new THREE.Color('#38bdf8'),
  sand: new THREE.Color('#fcd34d'),
  grass: new THREE.Color('#22c55e'),
  forest: new THREE.Color('#15803d'),
  mountain: new THREE.Color('#78716c'),
  snow: new THREE.Color('#f8fafc'),
};

// ========== SIMPLEX NOISE ==========

class SimplexNoise {
  private perm: Uint8Array;
  private grad3 = [
    [1, 1, 0],
    [-1, 1, 0],
    [1, -1, 0],
    [-1, -1, 0],
    [1, 0, 1],
    [-1, 0, 1],
    [1, 0, -1],
    [-1, 0, -1],
    [0, 1, 1],
    [0, -1, 1],
    [0, 1, -1],
    [0, -1, -1],
  ];

  constructor(seed = 0) {
    this.perm = new Uint8Array(512);
    const p = new Uint8Array(256);
    for (let i = 0; i < 256; i++) p[i] = i;
    let n = seed || Math.random() * 65536;
    for (let i = 255; i > 0; i--) {
      n = (n * 16807) % 2147483647;
      const j = n % (i + 1);
      [p[i], p[j]] = [p[j], p[i]];
    }
    for (let i = 0; i < 512; i++) this.perm[i] = p[i & 255];
  }

  noise2D(x: number, y: number): number {
    const F2 = 0.5 * (Math.sqrt(3) - 1);
    const G2 = (3 - Math.sqrt(3)) / 6;
    const s = (x + y) * F2;
    const i = Math.floor(x + s),
      j = Math.floor(y + s);
    const t = (i + j) * G2;
    const x0 = x - (i - t),
      y0 = y - (j - t);
    const i1 = x0 > y0 ? 1 : 0,
      j1 = x0 > y0 ? 0 : 1;
    const x1 = x0 - i1 + G2,
      y1 = y0 - j1 + G2;
    const x2 = x0 - 1 + 2 * G2,
      y2 = y0 - 1 + 2 * G2;
    const ii = i & 255,
      jj = j & 255;

    const dot = (gi: number, dx: number, dy: number) => {
      const g = this.grad3[gi % 12];
      return g[0] * dx + g[1] * dy;
    };

    let n0 = 0,
      n1 = 0,
      n2 = 0;
    let t0 = 0.5 - x0 * x0 - y0 * y0;
    if (t0 >= 0) {
      t0 *= t0;
      n0 = t0 * t0 * dot(this.perm[ii + this.perm[jj]], x0, y0);
    }
    let t1 = 0.5 - x1 * x1 - y1 * y1;
    if (t1 >= 0) {
      t1 *= t1;
      n1 = t1 * t1 * dot(this.perm[ii + i1 + this.perm[jj + j1]], x1, y1);
    }
    let t2 = 0.5 - x2 * x2 - y2 * y2;
    if (t2 >= 0) {
      t2 *= t2;
      n2 = t2 * t2 * dot(this.perm[ii + 1 + this.perm[jj + 1]], x2, y2);
    }
    return 70 * (n0 + n1 + n2);
  }

  fbm(x: number, y: number, octaves: number, persistence: number): number {
    let total = 0,
      amp = 1,
      freq = 1,
      max = 0;
    for (let i = 0; i < octaves; i++) {
      total += this.noise2D(x * freq, y * freq) * amp;
      max += amp;
      amp *= persistence;
      freq *= 2;
    }
    return total / max;
  }
}

// ========== TERRAIN MESH ==========

function TerrainMesh() {
  const sab = useSAB();
  const moduleExports = useSystemStore(s => s.moduleExports);
  const meshRef = useRef<THREE.Mesh>(null);
  const [seed] = useState(() => Math.floor(Math.random() * 10000));
  const noise = useMemo(() => new SimplexNoise(seed), [seed]);
  const lastGpuUpdate = useRef(0);
  const needsVertexUpdate = useRef(true);

  const tempColor = useMemo(() => new THREE.Color(), []);
  const localBuffer = useMemo(() => new Float32Array(CONFIG.GRID_SIZE * CONFIG.GRID_SIZE), []);

  const getHeightmap = () =>
    sab
      ? getArenaView(sab, SCENE_OFFSETS.TERRAIN_HEIGHTMAP, SCENE_OFFSETS.TERRAIN_SIZE)
      : localBuffer;

  const geometry = useMemo(() => {
    const geo = new THREE.PlaneGeometry(
      CONFIG.SCALE,
      CONFIG.SCALE,
      CONFIG.GRID_SIZE - 1,
      CONFIG.GRID_SIZE - 1
    );
    const count = geo.attributes.position.count;
    geo.setAttribute('color', new THREE.BufferAttribute(new Float32Array(count * 3), 3));
    return geo;
  }, []);

  const material = useMemo(
    () =>
      new THREE.MeshStandardMaterial({
        vertexColors: true,
        flatShading: true,
        side: THREE.DoubleSide,
        metalness: 0.05,
        roughness: 0.85,
      }),
    []
  );

  useEffect(
    () => () => {
      geometry.dispose();
      material.dispose();
    },
    [geometry, material]
  );

  useFrame(state => {
    if (!meshRef.current) return;

    const heightmap = getHeightmap();
    const positions = geometry.attributes.position;
    const colors = geometry.attributes.color;
    const time = state.clock.elapsedTime * CONFIG.ANIMATION_SPEED;

    // Dispatch 1: Terrain Perlin Base (Run periodically)
    const shouldGpuTerrain = moduleExports?.compute && time - lastGpuUpdate.current > 1.0;
    if (shouldGpuTerrain) {
      try {
        dispatch.execute('gpu', 'perlin_noise', {
          width: CONFIG.GRID_SIZE,
          height: CONFIG.GRID_SIZE,
          scale: 0.03,
          octaves: CONFIG.OCTAVES,
          persistence: CONFIG.PERSISTENCE,
          time: 0,
          sab_offset: SCENE_OFFSETS.TERRAIN_HEIGHTMAP,
        });
        lastGpuUpdate.current = time;
        needsVertexUpdate.current = true;
      } catch {
        // Fallback handled below
      }
    }

    // JS fallback: Simplex noise FBM (Run only on mount or once)
    if (!shouldGpuTerrain && lastGpuUpdate.current === 0) {
      for (let i = 0; i < CONFIG.GRID_SIZE; i++) {
        for (let j = 0; j < CONFIG.GRID_SIZE; j++) {
          heightmap[i * CONFIG.GRID_SIZE + j] = noise.fbm(
            (i / CONFIG.GRID_SIZE) * 4,
            (j / CONFIG.GRID_SIZE) * 4,
            CONFIG.OCTAVES,
            CONFIG.PERSISTENCE
          );
        }
      }
      lastGpuUpdate.current = 0.001; // Mark as done
      needsVertexUpdate.current = true;
    }

    // Update vertices with biome coloring only when needed
    if (needsVertexUpdate.current) {
      for (let i = 0; i < CONFIG.GRID_SIZE; i++) {
        for (let j = 0; j < CONFIG.GRID_SIZE; j++) {
          const idx = i * CONFIG.GRID_SIZE + j;
          const h = heightmap[idx];
          const nh = (h + 1) / 2; // Normalize to 0-1
          const worldHeight = h * CONFIG.HEIGHT_SCALE;

          // Clamp terrain below water level
          positions.setZ(idx, Math.max(worldHeight, CONFIG.WATER_LEVEL - 0.5));

          // Biome coloring with "personality"
          if (nh < 0.32) {
            tempColor.lerpColors(BIOME.sand, BIOME.sand, 0.5);
          } else if (nh < 0.38) {
            tempColor.lerpColors(BIOME.sand, BIOME.grass, (nh - 0.32) / 0.06);
          } else if (nh < 0.45) {
            tempColor.lerpColors(BIOME.grass, BIOME.forest, (nh - 0.38) / 0.07);
          } else if (nh < 0.6) {
            tempColor.lerpColors(BIOME.forest, BIOME.mountain, (nh - 0.45) / 0.15);
          } else if (nh < 0.75) {
            tempColor.lerpColors(BIOME.mountain, BIOME.snow, (nh - 0.6) / 0.15);
          } else {
            tempColor.copy(BIOME.snow);
          }

          colors.setXYZ(idx, tempColor.r, tempColor.g, tempColor.b);
        }
      }

      positions.needsUpdate = true;
      colors.needsUpdate = true;
      geometry.computeVertexNormals();
      needsVertexUpdate.current = false;
    }
  });

  return (
    <mesh ref={meshRef} rotation={[-Math.PI / 2, 0, 0]} position={[0, -8, 0]}>
      <primitive object={geometry} attach="geometry" />
      <primitive object={material} attach="material" />
    </mesh>
  );
}

// ========== CLOUD LAYER ==========

function CloudLayer() {
  const sab = useSAB();
  const moduleExports = useSystemStore(s => s.moduleExports);
  const meshRef = useRef<THREE.Mesh>(null);
  const lastUpdate = useRef(0);

  const geometry = useMemo(() => {
    const geo = new THREE.PlaneGeometry(
      CONFIG.CLOUD_SCALE,
      CONFIG.CLOUD_SCALE,
      CONFIG.CLOUD_GRID - 1,
      CONFIG.CLOUD_GRID - 1
    );
    return geo;
  }, []);

  const material = useMemo(
    () =>
      new THREE.MeshStandardMaterial({
        color: '#ffffff',
        transparent: true,
        opacity: 0.5,
        depthWrite: false,
        side: THREE.DoubleSide,
        roughness: 1,
      }),
    []
  );

  useFrame(state => {
    if (!meshRef.current || !sab) return;
    const time = state.clock.elapsedTime;
    const cloudMap = getArenaView(sab, SCENE_OFFSETS.CLOUD_MAP, SCENE_OFFSETS.CLOUD_SIZE);

    // Dispatch 2: Fractal Noise for animated clouds
    if (moduleExports?.compute && time - lastUpdate.current > 0.1) {
      dispatch.execute('gpu', 'fractal_noise', {
        width: CONFIG.CLOUD_GRID,
        height: CONFIG.CLOUD_GRID,
        scale: 0.05,
        octaves: 4,
        time: time * 0.1, // Slow movement
        sab_offset: SCENE_OFFSETS.CLOUD_MAP,
      });
      lastUpdate.current = time;
    }

    // Update geometry based on cloud noise
    const positions = geometry.attributes.position;
    for (let i = 0; i < CONFIG.CLOUD_GRID * CONFIG.CLOUD_GRID; i++) {
      const h = cloudMap[i];
      // Only raise cloud if noise value is positive
      positions.setZ(i, Math.max(0, h * 5));
    }
    positions.needsUpdate = true;
    geometry.computeVertexNormals();
  });

  return (
    <mesh
      ref={meshRef}
      geometry={geometry}
      material={material}
      rotation={[Math.PI / 2, 0, 0]}
      position={[0, CONFIG.CLOUD_HEIGHT, 0]}
    />
  );
}

// ========== WATER PLANE ==========

function WaterPlane() {
  const meshRef = useRef<THREE.Mesh>(null);

  // Water shader for realistic look
  const material = useMemo(
    () =>
      new THREE.MeshStandardMaterial({
        color: new THREE.Color('#0ea5e9'),
        transparent: true,
        opacity: 0.7,
        metalness: 0.1,
        roughness: 0.2,
        side: THREE.DoubleSide,
      }),
    []
  );

  useEffect(() => () => material.dispose(), [material]);

  useFrame(state => {
    if (!meshRef.current) return;
    // Gentle wave animation
    const time = state.clock.elapsedTime;
    meshRef.current.position.y = CONFIG.WATER_LEVEL + Math.sin(time * 0.5) * 0.05;
  });

  return (
    <mesh ref={meshRef} rotation={[-Math.PI / 2, 0, 0]} position={[0, -8 + CONFIG.WATER_LEVEL, 0]}>
      <planeGeometry args={[CONFIG.SCALE * 4, CONFIG.SCALE * 4, 1, 1]} />
      <primitive object={material} attach="material" />
    </mesh>
  );
}

// ========== SKY ==========

function SkyContent() {
  return (
    <>
      <Stars radius={100} depth={50} count={5000} factor={4} saturation={0} fade speed={1} />
    </>
  );
}

// ========== SCENE ==========

// ... (SimplexNoise class remains same)

function IntegratedWorldContent() {
  return (
    <>
      <color attach="background" args={['#38bdf8']} />
      <fog attach="fog" args={['#38bdf8', 10, 80]} />
      <ambientLight intensity={0.7} />
      <pointLight position={[10, 15, 10]} intensity={2.5} />
      <directionalLight position={[0, 20, 0]} intensity={1.2} />

      <TerrainMesh />

      <CloudLayer />

      <Float speed={1.5} rotationIntensity={0.2} floatIntensity={0.5}>
        <WaterPlane />
      </Float>

      <InstancedBoidsRenderer />

      <SkyContent />
    </>
  );
}

export default function TerrainScene({ isBackground = false }: { isBackground?: boolean }) {
  return (
    <SceneWrapper
      title="Zero-Copy Integrated World (gpu.rs + boids.rs)"
      showFPS={!isBackground}
      isBackground={isBackground}
    >
      <IntegratedWorldContent />
    </SceneWrapper>
  );
}
