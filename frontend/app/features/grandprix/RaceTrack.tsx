import { useMemo } from 'react';
import * as THREE from 'three';

export default function RaceTrack() {
  // Simple MultiGP style track
  // 8 gates in a simple circuit
  const gates = useMemo(() => {
    return [
      [0, 0, -10],
      [10, 0, -20],
      [20, 0, -10],
      [10, 0, 0],
      [0, 0, 10],
      [-10, 0, 20],
      [-20, 0, 10],
      [-10, 0, 0],
    ].map(pos => new THREE.Vector3(pos[0], pos[1], pos[2]));
  }, []);

  return (
    <group>
      {/* Floor grid */}
      <gridHelper args={[100, 100, 0x444444, 0x222222]} />

      {/* Gates */}
      {gates.map((pos, i) => {
        const nextPos = gates[(i + 1) % gates.length];
        const lookAtPos = nextPos.clone().sub(pos).normalize();
        const quaternion = new THREE.Quaternion().setFromUnitVectors(
          new THREE.Vector3(0, 0, 1),
          lookAtPos
        );

        return (
          <group key={i} position={pos} quaternion={quaternion}>
            <mesh position={[0, 1.5, 0]}>
              <torusGeometry args={[1.5, 0.1, 16, 32]} />
              <meshStandardMaterial color="#ffcc00" emissive="#ffaa00" emissiveIntensity={0.5} />
            </mesh>
            {/* Pole */}
            <mesh position={[1.5, 0.75, 0]}>
              <cylinderGeometry args={[0.05, 0.05, 1.5]} />
              <meshStandardMaterial color="#333" />
            </mesh>
            <mesh position={[-1.5, 0.75, 0]}>
              <cylinderGeometry args={[0.05, 0.05, 1.5]} />
              <meshStandardMaterial color="#333" />
            </mesh>
          </group>
        );
      })}
    </group>
  );
}
