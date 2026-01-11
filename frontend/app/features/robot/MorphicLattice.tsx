import React, { useRef, useMemo } from 'react';
import { useFrame } from '@react-three/fiber';
import * as THREE from 'three';
import { useLatticeState } from './useLatticeState';

// Static Colors for Zero-Allocation Rendering
const COLOR_CONNECTED_PULSE = new THREE.Color('#7c3aed'); // Violet-600
const COLOR_CONNECTED_BASE = new THREE.Color('#000000'); // Black
const COLOR_DISCONNECTED_PULSE = new THREE.Color('#ef4444'); // Red-500
const COLOR_DISCONNECTED_BASE = new THREE.Color('#450a0a'); // Red-950

/**
 * MorphicLattice - High-Performance Topological Manifold Visualization
 *
 * Renders 512 nodes via InstancedMesh and 1024 filaments via LineSegments.
 * State is pulled directly from SharedArrayBuffer for zero-copy 60FPS performance.
 */
export const MorphicLattice: React.FC = () => {
  const { metrics, matrices, filaments } = useLatticeState();
  const meshRef = useRef<THREE.InstancedMesh>(null);
  const lineRef = useRef<THREE.LineSegments>(null);

  // Initialize geometries
  const nodeGeometry = useMemo(() => new THREE.IcosahedronGeometry(0.1, 1), []);
  const lineGeometry = useMemo(() => {
    const geo = new THREE.BufferGeometry();
    const positions = new Float32Array(1024 * 2 * 3); // 1024 segments, 2 points each, 3 components
    geo.setAttribute('position', new THREE.BufferAttribute(positions, 3));
    return geo;
  }, []);

  // Explicit resource disposal (per graphics.md)
  React.useEffect(() => {
    return () => {
      nodeGeometry.dispose();
      lineGeometry.dispose();
    };
  }, [nodeGeometry, lineGeometry]);

  useFrame(() => {
    if (!matrices || !meshRef.current || !lineRef.current) return;

    // 1. Update Node Matrices
    const instanceMatrix = meshRef.current.instanceMatrix;
    instanceMatrix.needsUpdate = true;
    instanceMatrix.array.set(matrices);

    // 2. Update Filament Positions
    const lineGeo = lineRef.current.geometry;
    const positionAttr = lineGeo.attributes.position as THREE.BufferAttribute;
    const linePositions = positionAttr.array as Float32Array;

    if (filaments) {
      // Use raw loop for max performance (1024 iterations is trivial for JS)
      for (let i = 0; i < 1024; i++) {
        // Filaments buffer is Uint32Array [idx1, idx2] pairs
        const idx1 = filaments[i * 2];
        const idx2 = filaments[i * 2 + 1];

        // Ensure indices are within bounds (512 nodes)
        if (idx1 >= 512 || idx2 >= 512) continue;

        // Matrix Stride: 16 floats per node (64 bytes)
        // Check bounds safely
        const base1 = idx1 * 16;
        const base2 = idx2 * 16;

        if (base1 + 14 >= matrices.length || base2 + 14 >= matrices.length) continue;

        const i6 = i * 6;

        linePositions[i6 + 0] = matrices[base1 + 12];
        linePositions[i6 + 1] = matrices[base1 + 13];
        linePositions[i6 + 2] = matrices[base1 + 14];

        linePositions[i6 + 3] = matrices[base2 + 12];
        linePositions[i6 + 4] = matrices[base2 + 13];
        linePositions[i6 + 5] = matrices[base2 + 14];
      }
      positionAttr.needsUpdate = true;
    }

    // 3. Dynamic Coloring and Rotation
    if (metrics && lineRef.current) {
      const colorFactor = metrics.syntropy;

      // Pulse rotation
      meshRef.current.rotation.y += 0.001 + colorFactor * 0.002;
      lineRef.current.rotation.y = meshRef.current.rotation.y;

      // Pulse Filament Color
      const material = lineRef.current.material as THREE.LineBasicMaterial;
      const pulse = (Math.sin(Date.now() * 0.002) + 1) * 0.5; // 0..1

      // "Red Lighting" Logic for Disconnected/Searching State (Low Syntropy)
      // If syntropy is low (< 0.5), we are likely a single node searching for mesh.
      const isDisconnected = colorFactor < 0.3;

      const pulseColor = isDisconnected ? COLOR_DISCONNECTED_PULSE : COLOR_CONNECTED_PULSE;
      const baseColor = isDisconnected ? COLOR_DISCONNECTED_BASE : COLOR_CONNECTED_BASE;

      // Lerp Color
      material.color.lerpColors(baseColor, pulseColor, pulse * 0.6 + colorFactor * 0.4);

      // Lerp Opacity (Base 0.85 for clear visibility, Pulse to 1.0)
      material.opacity = 0.85 + pulse * 0.15;
    }
  });

  return (
    <group>
      {/* Structural Filaments (The Graph) */}
      <lineSegments ref={lineRef} geometry={lineGeometry}>
        <lineBasicMaterial
          transparent={true}
          opacity={0.85} // Higher base opacity
          color="#000000" // Base Black
          linewidth={2} // Thicker lines
          blending={THREE.NormalBlending}
          depthWrite={false}
        />
      </lineSegments>

      {/* Compute Nodes (The Manifold) - Spheres */}
      <instancedMesh ref={meshRef} args={[nodeGeometry, undefined, 512]} frustumCulled={false}>
        <meshPhysicalMaterial
          color="#0f172a" // Slate-900 (Black/Dark Blue)
          emissive="#000000"
          metalness={0.5}
          roughness={0.5}
          transparent
          opacity={1.0}
        />
      </instancedMesh>

      {/* Core Glow - Subtle on white */}
      {/* <pointLight position={[0, 0, 0]} intensity={2.0} distance={15} color="#8b5cf6" /> */}
    </group>
  );
};
