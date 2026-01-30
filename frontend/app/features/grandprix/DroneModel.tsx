import { useMemo, useRef } from 'react';
import { useFrame } from '@react-three/fiber';
import * as THREE from 'three';
import { useSystemStore } from '../../../src/store/system';
import { SAB_OFFSETS, DRONE_CONSTANTS } from '../../../src/racing/layout';

const dummyObject = new THREE.Object3D();
const dummyColor = new THREE.Color();

export default function DroneModel() {
  const sab = useSystemStore(s => s.sab);
  const meshRef = useRef<THREE.InstancedMesh>(null);

  // Buffer views
  const stateView = useMemo(() => {
    if (!sab) return null;
    return new Float32Array(
      sab,
      SAB_OFFSETS.DRONE_STATE_A,
      DRONE_CONSTANTS.MAX_DRONES * (DRONE_CONSTANTS.STRIDE / 4)
    );
  }, [sab]);

  // Visuals using simple box for now (resembling Neros 8-inch logic)
  // X-configuration: Central body + 4 arms?
  // For instanced mesh, we need a single geometry. A simple cone or flattened box works for orientation.
  const geometry = useMemo(() => {
    const geo = new THREE.ConeGeometry(0.2, 0.5, 4);
    geo.rotateX(Math.PI / 2); // Point forward
    return geo;
  }, []);

  const material = useMemo(() => {
    return new THREE.MeshStandardMaterial({
      color: '#00ff88',
      roughness: 0.4,
      metalness: 0.8,
      emissive: '#004422',
      emissiveIntensity: 0.5,
    });
  }, []);

  useFrame(() => {
    if (!meshRef.current || !stateView) return;

    for (let i = 0; i < DRONE_CONSTANTS.MAX_DRONES; i++) {
      // Only render active drones? For Phase 1 we have 8 drones initialized.
      // We can check if position is non-zero or just render all.

      const base = i * (DRONE_CONSTANTS.STRIDE / 4);
      const px = stateView[base + 0];
      const py = stateView[base + 1];
      const pz = stateView[base + 2];

      // Orientation
      const qw = stateView[base + 8];
      const qx = stateView[base + 9];
      const qy = stateView[base + 10];
      const qz = stateView[base + 11];

      // Apply to dummy
      dummyObject.position.set(px, py, pz);
      dummyObject.quaternion.set(qx, qy, qz, qw);
      dummyObject.updateMatrix();

      meshRef.current.setMatrixAt(i, dummyObject.matrix);

      // Color based on index for visibility
      if (i === 0)
        dummyColor.set('#ff0055'); // Hero drone
      else dummyColor.set('#00ff88');
      meshRef.current.setColorAt(i, dummyColor);
    }
    meshRef.current.instanceMatrix.needsUpdate = true;
    if (meshRef.current.instanceColor) meshRef.current.instanceColor.needsUpdate = true;
  });

  return (
    <instancedMesh
      ref={meshRef}
      args={[geometry, material, DRONE_CONSTANTS.MAX_DRONES]}
      frustumCulled={false}
    />
  );
}
