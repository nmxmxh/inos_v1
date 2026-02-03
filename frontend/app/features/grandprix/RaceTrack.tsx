/**
 * Premium LED Race Track - Production-grade environment
 *
 * Architecture aligned with graphics.md:
 * - Cached GPU noise textures (no per-frame allocations)
 * - GPU unit for procedural noise (execute_wgsl)
 * - Epoch-safe render loop
 */

import { useEffect, useMemo, useRef } from 'react';
import { useFrame } from '@react-three/fiber';
import * as THREE from 'three';
import { dispatch } from '../../../src/wasm/dispatch';
import { WebGpuExecutor } from '../../../src/wasm/gpu/WebGpuExecutor';
import type { WebGpuRequest } from '../../../src/wasm/gpu/ShaderPipelineManager';

const GATE_COLORS = [
  '#e11d48',
  '#ea580c',
  '#ca8a04',
  '#16a34a',
  '#0891b2',
  '#2563eb',
  '#7c3aed',
  '#c026d3',
];

const GATE_POSITIONS: [number, number, number][] = [
  [0, 3, -20],
  [20, 4, -35],
  [40, 3, -20],
  [20, 4, 0],
  [0, 3, 20],
  [-20, 4, 35],
  [-40, 3, 20],
  [-20, 4, 0],
];

const NOISE_SIZE = 128;
const NOISE_FLOATS = NOISE_SIZE * NOISE_SIZE;
const NOISE_DISPATCH = [Math.ceil(NOISE_SIZE / 8), Math.ceil(NOISE_SIZE / 8), 1] as const;
const NOISE_WORKGROUP = [8, 8, 1] as const;
const NOISE_UPDATE_INTERVAL = 6.0;

const NOISE_WGSL = `
@group(0) @binding(0) var<storage, read> input: array<f32>;
@group(0) @binding(1) var<storage, read_write> output: array<f32>;

const WIDTH: u32 = ${NOISE_SIZE}u;
const HEIGHT: u32 = ${NOISE_SIZE}u;

fn hash(p: vec2<f32>) -> f32 {
  let h = dot(p, vec2<f32>(127.1, 311.7));
  return fract(sin(h) * 43758.5453123);
}

fn noise(p: vec2<f32>) -> f32 {
  let i = floor(p);
  let f = fract(p);
  let a = hash(i);
  let b = hash(i + vec2<f32>(1.0, 0.0));
  let c = hash(i + vec2<f32>(0.0, 1.0));
  let d = hash(i + vec2<f32>(1.0, 1.0));
  let u = f * f * (3.0 - 2.0 * f);
  return mix(mix(a, b, u.x), mix(c, d, u.x), u.y);
}

fn fbm(p: vec2<f32>) -> f32 {
  var sum = 0.0;
  var amp = 0.55;
  var freq = 1.0;
  for (var i = 0; i < 4; i = i + 1) {
    sum = sum + noise(p * freq) * amp;
    amp = amp * 0.5;
    freq = freq * 2.0;
  }
  return sum;
}

@compute @workgroup_size(8, 8, 1)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
  if (gid.x >= WIDTH || gid.y >= HEIGHT) {
    return;
  }
  let idx = gid.y * WIDTH + gid.x;
  let time = input[0];
  let scale = input[1];
  let offsetX = input[2];
  let offsetY = input[3];
  let uv = vec2<f32>(f32(gid.x) / f32(WIDTH), f32(gid.y) / f32(HEIGHT));
  let p = uv * scale + vec2<f32>(offsetX, offsetY) + vec2<f32>(time * 0.03, time * 0.015);
  output[idx] = fbm(p);
}
`;

const tempColor = new THREE.Color();

function updateTextures(
  values: Float32Array,
  groundBytes: Uint8Array,
  cloudBytes: Uint8Array,
  weather: WeatherState
) {
  const sample0 = values[0] || 0.0;
  const sample1 = values[Math.floor(NOISE_FLOATS / 3)] || 0.0;
  const sample2 = values[Math.floor(NOISE_FLOATS / 2)] || 0.0;

  weather.wind = 4 + sample0 * 8;
  weather.humidity = 0.3 + sample1 * 0.6;
  weather.visibility = 0.5 + sample2 * 0.5;
  weather.temperature = 22 + sample0 * 6;

  for (let i = 0; i < NOISE_FLOATS; i++) {
    const v = values[i];
    const grass = Math.min(1, Math.max(0, 0.35 + v * 0.75));
    const darken = 0.55 + v * 0.35;
    const base = i * 3;
    groundBytes[base] = Math.floor(50 * darken + 40);
    groundBytes[base + 1] = Math.floor(120 * grass + 50);
    groundBytes[base + 2] = Math.floor(50 * darken + 40);

    const cloud = Math.max(0, Math.min(1, (v - 0.45) * 2.2));
    cloudBytes[i] = Math.floor(cloud * 255);
  }
}

function SkyDome({ sunDir }: { sunDir: THREE.Vector3 }) {
  const material = useMemo(
    () =>
      new THREE.ShaderMaterial({
        side: THREE.BackSide,
        depthWrite: false,
        uniforms: {
          topColor: { value: new THREE.Color('#0b172a') },
          horizonColor: { value: new THREE.Color('#7dd3fc') },
          bottomColor: { value: new THREE.Color('#f8fafc') },
          sunDir: { value: sunDir.clone() },
          sunIntensity: { value: 1.2 },
          haze: { value: 0.45 },
        },
        vertexShader: `
          varying vec3 vWorldPosition;
          void main() {
            vec4 worldPosition = modelMatrix * vec4(position, 1.0);
            vWorldPosition = worldPosition.xyz;
            gl_Position = projectionMatrix * viewMatrix * worldPosition;
          }
        `,
        fragmentShader: `
          varying vec3 vWorldPosition;
          uniform vec3 topColor;
          uniform vec3 horizonColor;
          uniform vec3 bottomColor;
          uniform vec3 sunDir;
          uniform float sunIntensity;
          uniform float haze;

          void main() {
            vec3 dir = normalize(vWorldPosition);
            float h = clamp(dir.y * 0.5 + 0.5, 0.0, 1.0);
            vec3 sky = mix(bottomColor, horizonColor, smoothstep(0.0, 0.55, h));
            sky = mix(sky, topColor, smoothstep(0.4, 1.0, h));

            float sunDot = max(dot(dir, normalize(sunDir)), 0.0);
            float sun = pow(sunDot, 400.0) * sunIntensity;
            float glow = pow(sunDot, 6.0) * 0.35;

            sky += vec3(1.0, 0.9, 0.7) * (sun + glow);
            sky = mix(sky, horizonColor, haze * (1.0 - h));
            gl_FragColor = vec4(sky, 1.0);
          }
        `,
      }),
    [sunDir]
  );

  useFrame(() => {
    material.uniforms.sunDir.value.copy(sunDir);
  });

  return (
    <mesh scale={300} material={material}>
      <sphereGeometry args={[1, 32, 32]} />
    </mesh>
  );
}

function Gate({
  position,
  index,
  lookAt,
}: {
  position: [number, number, number];
  index: number;
  lookAt: THREE.Vector3;
}) {
  const color = GATE_COLORS[index % GATE_COLORS.length];

  const rotation = useMemo(() => {
    const dir = new THREE.Vector3().subVectors(lookAt, new THREE.Vector3(...position));
    dir.y = 0;
    const angle = Math.atan2(dir.x, dir.z);
    return [0, angle, 0] as [number, number, number];
  }, [position, lookAt]);

  return (
    <group position={position} rotation={rotation}>
      <mesh position={[-2.5, -1.5, 0]} castShadow>
        <cylinderGeometry args={[0.12, 0.15, 3, 16]} />
        <meshStandardMaterial color="#374151" metalness={0.7} roughness={0.3} />
      </mesh>
      <mesh position={[2.5, -1.5, 0]} castShadow>
        <cylinderGeometry args={[0.12, 0.15, 3, 16]} />
        <meshStandardMaterial color="#374151" metalness={0.7} roughness={0.3} />
      </mesh>
      <mesh position={[0, 0.6, 0]} castShadow>
        <boxGeometry args={[5.3, 0.25, 0.25]} />
        <meshStandardMaterial color="#374151" metalness={0.7} roughness={0.3} />
      </mesh>
      <mesh position={[0, -0.5, 0.05]}>
        <torusGeometry args={[2.2, 0.1, 8, 32]} />
        <meshStandardMaterial
          color={color}
          emissive={color}
          emissiveIntensity={1.6}
          toneMapped={false}
        />
      </mesh>
      <mesh position={[0, -0.5, 0]}>
        <ringGeometry args={[1.8, 2.4, 32]} />
        <meshBasicMaterial color={color} transparent opacity={0.12} side={THREE.DoubleSide} />
      </mesh>
      <mesh position={[0, 1.2, 0]} castShadow>
        <boxGeometry args={[1.2, 0.6, 0.08]} />
        <meshStandardMaterial color="#1f2937" metalness={0.8} roughness={0.2} />
      </mesh>
      <pointLight color={color} intensity={2} distance={12} position={[0, -0.5, 0.5]} />
    </group>
  );
}

type WeatherState = {
  wind: number;
  humidity: number;
  visibility: number;
  temperature: number;
};

function GroundPlane({
  groundTexture,
  cloudTexture,
}: {
  groundTexture: THREE.DataTexture;
  cloudTexture: THREE.DataTexture;
}) {
  const groundMaterial = useMemo(
    () =>
      new THREE.MeshStandardMaterial({
        color: '#4ade80',
        roughness: 0.88,
        metalness: 0.05,
        map: groundTexture,
        roughnessMap: groundTexture,
      }),
    [groundTexture]
  );

  const asphaltMaterial = useMemo(
    () =>
      new THREE.MeshStandardMaterial({
        color: '#334155',
        roughness: 0.75,
        metalness: 0.2,
        map: groundTexture,
      }),
    [groundTexture]
  );

  const cloudMaterial = useMemo(
    () =>
      new THREE.MeshBasicMaterial({
        color: '#0f172a',
        transparent: true,
        opacity: 0.18,
        alphaMap: cloudTexture,
        depthWrite: false,
      }),
    [cloudTexture]
  );

  return (
    <group>
      <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, -0.01, 0]} receiveShadow>
        <planeGeometry args={[240, 240, 1, 1]} />
        <primitive object={groundMaterial} attach="material" />
      </mesh>

      <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, 0, 0]} receiveShadow>
        <ringGeometry args={[15, 50, 64]} />
        <primitive object={asphaltMaterial} attach="material" />
      </mesh>

      <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, 0.02, 0]} receiveShadow>
        <circleGeometry args={[8, 32]} />
        <meshStandardMaterial color="#64748b" roughness={0.6} metalness={0.2} />
      </mesh>

      <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, 12, 0]}>
        <planeGeometry args={[220, 220, 1, 1]} />
        <primitive object={cloudMaterial} attach="material" />
      </mesh>
    </group>
  );
}

function StadiumLights({ weather, sunDir }: { weather: WeatherState; sunDir: THREE.Vector3 }) {
  const keyLightRef = useRef<THREE.DirectionalLight>(null);
  const fillLightRef = useRef<THREE.DirectionalLight>(null);
  const hemiRef = useRef<THREE.HemisphereLight>(null);

  useFrame(state => {
    const t = state.clock.elapsedTime;
    const sunAngle = t * 0.03;
    sunDir.set(Math.sin(sunAngle) * 0.6, 0.7 + Math.sin(sunAngle * 0.5) * 0.1, Math.cos(sunAngle));

    if (keyLightRef.current) {
      keyLightRef.current.position.set(60, 80, 30);
      keyLightRef.current.intensity = 1.8 + weather.visibility * 0.4;
      keyLightRef.current.color.set(tempColor.setHSL(0.58, 0.35, 0.75));
    }

    if (fillLightRef.current) {
      fillLightRef.current.intensity = 0.4 + weather.humidity * 0.2;
    }

    if (hemiRef.current) {
      hemiRef.current.intensity = 0.35 + weather.humidity * 0.2;
    }
  });

  return (
    <>
      <directionalLight
        ref={keyLightRef}
        position={[50, 80, 30]}
        intensity={2}
        castShadow
        shadow-mapSize-width={2048}
        shadow-mapSize-height={2048}
        shadow-camera-far={200}
        shadow-camera-left={-80}
        shadow-camera-right={80}
        shadow-camera-top={80}
        shadow-camera-bottom={-80}
      />
      <directionalLight ref={fillLightRef} position={[-30, 40, -20]} intensity={0.55} />
      <ambientLight intensity={0.28} />
      <hemisphereLight ref={hemiRef} args={['#93c5fd', '#4ade80', 0.4]} />
    </>
  );
}

export default function RaceTrack() {
  const sunDir = useMemo(() => new THREE.Vector3(0.4, 0.8, 0.2), []);
  const weatherRef = useRef<WeatherState>({
    wind: 6,
    humidity: 0.45,
    visibility: 0.8,
    temperature: 24,
  });
  const fogRef = useRef<THREE.FogExp2 | null>(null);

  const noiseInput = useMemo(() => new Float32Array(NOISE_FLOATS), []);
  const noiseInputBytes = useMemo(() => new Uint8Array(noiseInput.buffer), [noiseInput]);
  const decoderRef = useRef(new TextDecoder());
  const gpuExecutorRef = useRef<WebGpuExecutor | null>(null);
  const gpuBusyRef = useRef(false);
  const lastNoiseUpdateRef = useRef(0);

  const groundBytes = useMemo(() => new Uint8Array(NOISE_FLOATS * 3), []);
  const cloudBytes = useMemo(() => new Uint8Array(NOISE_FLOATS), []);

  const groundTexture = useMemo(() => {
    const texture = new THREE.DataTexture(
      groundBytes,
      NOISE_SIZE,
      NOISE_SIZE,
      THREE.RGBFormat
    );
    texture.colorSpace = THREE.SRGBColorSpace;
    texture.wrapS = THREE.RepeatWrapping;
    texture.wrapT = THREE.RepeatWrapping;
    texture.repeat.set(14, 14);
    texture.minFilter = THREE.LinearFilter;
    texture.magFilter = THREE.LinearFilter;
    texture.generateMipmaps = false;
    return texture;
  }, [groundBytes]);

  const cloudTexture = useMemo(() => {
    const texture = new THREE.DataTexture(
      cloudBytes,
      NOISE_SIZE,
      NOISE_SIZE,
      THREE.RedFormat
    );
    texture.wrapS = THREE.RepeatWrapping;
    texture.wrapT = THREE.RepeatWrapping;
    texture.repeat.set(3, 3);
    texture.minFilter = THREE.LinearFilter;
    texture.magFilter = THREE.LinearFilter;
    texture.generateMipmaps = false;
    return texture;
  }, [cloudBytes]);

  const gates = useMemo(() => {
    return GATE_POSITIONS.map((pos, i) => {
      const nextPos = GATE_POSITIONS[(i + 1) % GATE_POSITIONS.length];
      return {
        position: pos,
        lookAt: new THREE.Vector3(...nextPos),
      };
    });
  }, []);

  useEffect(() => {
    updateTextures(
      new Float32Array(NOISE_FLOATS).fill(0.45),
      groundBytes,
      cloudBytes,
      weatherRef.current
    );
    groundTexture.needsUpdate = true;
    cloudTexture.needsUpdate = true;
  }, [cloudBytes, cloudTexture, groundBytes, groundTexture]);

  useFrame(state => {
    const time = state.clock.elapsedTime;
    const weather = weatherRef.current;

    if (cloudTexture) {
      cloudTexture.offset.x = (time * weather.wind * 0.002) % 1;
      cloudTexture.offset.y = (time * weather.wind * 0.001) % 1;
    }

    if (fogRef.current) {
      const density = 0.008 + (1 - weather.visibility) * 0.015 + weather.humidity * 0.004;
      fogRef.current.density = density;
      fogRef.current.color.setHSL(0.58, 0.45, 0.88 - weather.humidity * 0.25);
    }

    if (time - lastNoiseUpdateRef.current < NOISE_UPDATE_INTERVAL || gpuBusyRef.current) {
      return;
    }

    if (!dispatch.has('gpu', 'execute_wgsl')) {
      lastNoiseUpdateRef.current = time;
      return;
    }

    gpuBusyRef.current = true;
    noiseInput[0] = time;
    noiseInput[1] = 6.0;
    noiseInput[2] = Math.sin(time * 0.04) * 0.5;
    noiseInput[3] = Math.cos(time * 0.03) * 0.5;

    const run = async () => {
      try {
        if (!dispatch.has('gpu', 'execute_wgsl')) {
          return;
        }

        const response = await dispatch.execute(
          'gpu',
          'execute_wgsl',
          {
            shader: NOISE_WGSL,
            workgroup: NOISE_WORKGROUP,
            dispatch: NOISE_DISPATCH,
            buffer_type: 'float32',
          },
          noiseInputBytes
        );

        if (!response) return;

        const request = JSON.parse(
          decoderRef.current.decode(response)
        ) as WebGpuRequest;

        if (!gpuExecutorRef.current) {
          gpuExecutorRef.current = new WebGpuExecutor();
        }

        const output = await gpuExecutorRef.current.execute(request);
        const values = new Float32Array(output.buffer, output.byteOffset, output.byteLength / 4);
        updateTextures(values, groundBytes, cloudBytes, weather);

        groundTexture.needsUpdate = true;
        cloudTexture.needsUpdate = true;
        lastNoiseUpdateRef.current = time;
      } catch {
        // Keep last valid textures in place
      } finally {
        gpuBusyRef.current = false;
      }
    };

    void run();
  });

  return (
    <group>
      <SkyDome sunDir={sunDir} />
      <fogExp2 ref={fogRef} attach="fog" args={['#dbeafe', 0.012]} />

      <GroundPlane groundTexture={groundTexture} cloudTexture={cloudTexture} />
      <gridHelper args={[200, 100, '#94a3b8', '#cbd5e1']} position={[0, 0.02, 0]} />

      <StadiumLights weather={weatherRef.current} sunDir={sunDir} />

      {gates.map((gate, i) => (
        <Gate key={i} position={gate.position} index={i} lookAt={gate.lookAt} />
      ))}

      <group position={[0, 5, -20]}>
        <mesh>
          <boxGeometry args={[8, 1, 0.2]} />
          <meshStandardMaterial color="#0f172a" metalness={0.85} roughness={0.2} />
        </mesh>
        <mesh position={[0, 0, 0.11]}>
          <planeGeometry args={[7.5, 0.7]} />
          <meshBasicMaterial color="#fbbf24" />
        </mesh>
      </group>

      {[-60, 60].map(x =>
        [-60, 60].map(z => (
          <mesh key={`${x}-${z}`} position={[x, 0.5, z]}>
            <cylinderGeometry args={[0.3, 0.3, 1, 8]} />
            <meshStandardMaterial color="#f97316" />
          </mesh>
        ))
      )}
    </group>
  );
}
