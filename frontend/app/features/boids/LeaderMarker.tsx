import { useRef } from 'react';
import { useFrame } from '@react-three/fiber';
import { Billboard, Text } from '@react-three/drei';
import * as THREE from 'three';

interface LeaderMarkerProps {
  sab: SharedArrayBuffer;
  birdIndex: number;
  offset: number;
}

export default function LeaderMarker({ sab, birdIndex }: LeaderMarkerProps) {
  const groupRef = useRef<THREE.Group>(null);
  const ringRef = useRef<THREE.Mesh>(null);

  // Refs for dynamic elements
  const energyBarRef = useRef<THREE.Mesh>(null);
  const fitnessTextRef = useRef<any>(null); // Troika text instance
  const ringMatRef = useRef<THREE.MeshBasicMaterial>(null);
  const energyTextRef = useRef<any>(null);

  // Reusable vector for position reading
  const targetPos = useRef(new THREE.Vector3());

  useFrame(() => {
    // 1. Read Data Directly from SAB (Zero-Copy-ish)
    // Layout: [0-2] pos, [10] energy, [14] fitness, [18] trick
    const birdStrideFloat = 59;
    const birdOffsetFloat = 0x400000 / 4 + birdIndex * birdStrideFloat;

    // Create view
    const f32 = new Float32Array(sab);

    // Position Interpolation
    targetPos.current.set(
      f32[birdOffsetFloat + 0],
      f32[birdOffsetFloat + 1],
      f32[birdOffsetFloat + 2]
    );

    // Check if active (if pos is exactly 0,0,0 and energy is 0, likely uninitialized)
    const energy = f32[birdOffsetFloat + 10];
    const isActive = !(
      f32[birdOffsetFloat] === 0 &&
      f32[birdOffsetFloat + 1] === 0 &&
      f32[birdOffsetFloat + 2] === 0 &&
      energy === 0
    );

    if (groupRef.current) {
      groupRef.current.visible = isActive;
      if (isActive) {
        groupRef.current.position.lerp(targetPos.current, 0.2);
      }
    }

    if (!isActive) return;

    // Ring Animation
    if (ringRef.current) {
      ringRef.current.rotation.z += 0.02;
      ringRef.current.rotation.x = Math.sin(Date.now() * 0.002) * 0.2;
    }

    // Update Energy Bar
    if (energyBarRef.current) {
      // Max width is 0.6.
      // We use a 1.0 wide bar and scale it.
      const targetWidth = 0.6 * Math.max(0.01, energy);
      energyBarRef.current.scale.x = targetWidth;
      // Shift position to simulate left anchor
      // Center moves to targetWidth / 2
      energyBarRef.current.position.x = targetWidth / 2;
    }

    // Update Text Colors / Content
    const fitness = f32[birdOffsetFloat + 14];
    if (fitnessTextRef.current) {
      fitnessTextRef.current.text = `FIT ${fitness.toFixed(2)}`;
    }

    // Update Colors based on state
    const isTrick = f32[birdOffsetFloat + 18] > 0.01;
    if (ringMatRef.current) {
      ringMatRef.current.color.set(isTrick ? '#f59e0b' : '#8b5cf6');
    }

    const energyColor = energy < 0.3 ? '#ef4444' : '#10b981';
    if (energyTextRef.current) {
      energyTextRef.current.color = energyColor;
    }
    if (energyBarRef.current) {
      (energyBarRef.current.material as THREE.MeshBasicMaterial).color.set(energyColor);
    }
  });

  return (
    <group ref={groupRef}>
      {/* Target Lock Ring */}
      <mesh ref={ringRef} rotation={[Math.PI / 2, 0, 0]}>
        <torusGeometry args={[0.8, 0.02, 8, 32]} />
        <meshBasicMaterial ref={ringMatRef} color="#8b5cf6" transparent opacity={0.6} />
      </mesh>

      {/* Floating Stats */}
      <Billboard follow={true} lockX={false} lockY={false} lockZ={false}>
        <group position={[0, 1.2, 0]}>
          {/* Energy Icon/Bar */}
          <group position={[-0.4, 0, 0]}>
            <Text
              ref={energyTextRef}
              fontSize={0.25}
              color="#10b981"
              font="/fonts/InterDisplay-Bold.ttf"
              anchorX="right"
            >
              ⚡
            </Text>

            {/* Bar Container - Anchor Point */}
            <group position={[0.1, 0, 0]}>
              {/* Background Bar (Fixed 0.6 width, centered at 0.3) */}
              <mesh position={[0.3, 0, -0.01]}>
                <planeGeometry args={[0.6, 0.08]} />
                <meshBasicMaterial color="#334155" />
              </mesh>

              {/* Active Energy Bar (Dynamic Scale/Pos) */}
              {/* Init at scale 1, pos 0.5 (center of 1 unit). We manage this in useFrame. */}
              <mesh ref={energyBarRef} position={[0.3, 0, 0]}>
                <planeGeometry args={[1.0, 0.08]} />
                <meshBasicMaterial color="#10b981" />
              </mesh>
            </group>
          </group>

          {/* Fitness Score */}
          <group position={[0.4, 0, 0]}>
            <Text
              ref={fitnessTextRef}
              fontSize={0.2}
              color="#ffffff"
              font="/fonts/InterDisplay-Bold.ttf"
              anchorX="left"
            >
              FIT 0.00
            </Text>
          </group>
        </group>
      </Billboard>

      {/* Connection Line to Bird */}
      <mesh position={[0, 0.6, 0]}>
        <cylinderGeometry args={[0.01, 0.01, 1.2]} />
        <meshBasicMaterial color="#8b5cf6" transparent opacity={0.3} />
      </mesh>
    </group>
  );
}
