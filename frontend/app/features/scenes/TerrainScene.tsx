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
import { Sky, Environment, Stars } from '@react-three/drei';
import SceneWrapper, { useSAB, SCENE_OFFSETS, getArenaView } from './SceneWrapper';
import { useSystemStore } from '../../../src/store/system';
import { dispatch } from '../../../src/wasm/dispatch';

// ========== CONFIGURATION ==========

const CONFIG = {
  GRID_SIZE: 100,
  SCALE: 60,
  HEIGHT_SCALE: 8,
  WATER_LEVEL: -3.5,
  OCTAVES: 6,
  PERSISTENCE: 0.45,
  ANIMATION_SPEED: 0.02,
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
  const { moduleExports } = useSystemStore();
  const meshRef = useRef<THREE.Mesh>(null);
  const [seed] = useState(() => Math.floor(Math.random() * 10000));
  const noise = useMemo(() => new SimplexNoise(seed), [seed]);
  const lastGpuUpdate = useRef(0);

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

    // Try GPU dispatch for perlin_noise
    const shouldGpu = moduleExports?.compute && time - lastGpuUpdate.current > 0.1;
    if (shouldGpu) {
      try {
        const result = dispatch.execute('gpu', 'perlin_noise', {
          width: CONFIG.GRID_SIZE,
          height: CONFIG.GRID_SIZE,
          scale: 0.03,
          octaves: CONFIG.OCTAVES,
          persistence: CONFIG.PERSISTENCE,
          time,
          sab_offset: SCENE_OFFSETS.TERRAIN_HEIGHTMAP,
        });
        if (result?.length) {
          heightmap.set(new Float32Array(result.buffer).subarray(0, heightmap.length));
          lastGpuUpdate.current = time;
        }
      } catch {
        /* JS fallback */
      }
    }

    // JS fallback: Simplex noise FBM
    if (!shouldGpu || time - lastGpuUpdate.current > 0.15) {
      for (let i = 0; i < CONFIG.GRID_SIZE; i++) {
        for (let j = 0; j < CONFIG.GRID_SIZE; j++) {
          heightmap[i * CONFIG.GRID_SIZE + j] = noise.fbm(
            (i / CONFIG.GRID_SIZE) * 4 + time * 0.3,
            (j / CONFIG.GRID_SIZE) * 4,
            CONFIG.OCTAVES,
            CONFIG.PERSISTENCE
          );
        }
      }
    }

    // Update vertices with biome coloring
    for (let i = 0; i < CONFIG.GRID_SIZE; i++) {
      for (let j = 0; j < CONFIG.GRID_SIZE; j++) {
        const idx = i * CONFIG.GRID_SIZE + j;
        const h = heightmap[idx];
        const nh = (h + 1) / 2; // Normalize to 0-1
        const worldHeight = h * CONFIG.HEIGHT_SCALE;

        // Clamp terrain below water level
        positions.setZ(idx, Math.max(worldHeight, CONFIG.WATER_LEVEL - 0.5));

        // Biome coloring based on height
        if (nh < 0.32) {
          // Underwater - use sand color (visible through water)
          tempColor.lerpColors(BIOME.sand, BIOME.sand, 0.5);
        } else if (nh < 0.38) {
          // Beach/sand
          tempColor.lerpColors(BIOME.sand, BIOME.grass, (nh - 0.32) / 0.06);
        } else if (nh < 0.5) {
          // Grassland
          tempColor.lerpColors(BIOME.grass, BIOME.forest, (nh - 0.38) / 0.12);
        } else if (nh < 0.65) {
          // Forest
          tempColor.lerpColors(BIOME.forest, BIOME.mountain, (nh - 0.5) / 0.15);
        } else if (nh < 0.8) {
          // Mountain rock
          tempColor.lerpColors(BIOME.mountain, BIOME.snow, (nh - 0.65) / 0.15);
        } else {
          // Snow caps
          tempColor.copy(BIOME.snow);
        }

        colors.setXYZ(idx, tempColor.r, tempColor.g, tempColor.b);
      }
    }

    positions.needsUpdate = true;
    colors.needsUpdate = true;
    geometry.computeVertexNormals();
  });

  return (
    <mesh ref={meshRef} rotation={[-Math.PI / 2, 0, 0]} position={[0, -8, 0]}>
      <primitive object={geometry} attach="geometry" />
      <primitive object={material} attach="material" />
    </mesh>
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
      <planeGeometry args={[CONFIG.SCALE * 1.5, CONFIG.SCALE * 1.5, 1, 1]} />
      <primitive object={material} attach="material" />
    </mesh>
  );
}

// ========== SKY ==========

function SkyContent() {
  return (
    <>
      <Sky
        distance={450000}
        sunPosition={[100, 20, 100]}
        inclination={0}
        azimuth={0.25}
        turbidity={10}
        rayleigh={2}
      />
      <Stars radius={100} depth={50} count={5000} factor={4} saturation={0} fade speed={1} />
      <Environment preset="sunset" />
    </>
  );
}

// ========== SCENE ==========

// ... (SimplexNoise class remains same)

function IntegratedWorldContent() {
  return (
    <>
      <ambientLight intensity={0.4} />
      <pointLight position={[10, 10, 10]} intensity={1} />
      <directionalLight position={[0, 10, 0]} intensity={0.5} />

      <TerrainMesh />
      <WaterPlane />
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
