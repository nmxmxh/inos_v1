/**
 * Premium Racing Drone Model
 *
 * Architecture-aligned with graphics.md:
 * - Cached SAB views (no per-frame allocation)
 * - Epoch-driven ping-pong reads
 * - Instanced meshes for all parts
 */

import { useEffect, useMemo, useRef } from 'react';
import { useFrame } from '@react-three/fiber';
import * as THREE from 'three';
import { useSystemStore } from '../../../src/store/system';
import { SAB_OFFSETS, DRONE_CONSTANTS, EPOCH_INDICES } from '../../../src/racing/layout';

const DRONE_COLORS = [
  '#ef4444',
  '#3b82f6',
  '#eab308',
  '#22c55e',
  '#f97316',
  '#a855f7',
  '#ec4899',
  '#06b6d4',
];

const MAX_MOTOR_RPM = 35000;
// Motor order: front-right, front-left, back-left, back-right (matches physics)
const ARM_ANGLES = [
  (7 * Math.PI) / 4,
  (5 * Math.PI) / 4,
  (3 * Math.PI) / 4,
  Math.PI / 4,
];
const ARM_LENGTH = 0.11;
const MOTOR_RADIUS = 0.016;
const PROP_RADIUS = 0.0635;
const RPM_TO_RAD = (Math.PI * 2) / 60;
const MOTOR_DIR = [1, -1, 1, -1];

const dummyObject = new THREE.Object3D();
const tempQuat = new THREE.Quaternion();
const tempVec = new THREE.Vector3();
const tempColor = new THREE.Color();

export default function DroneModel() {
  const sab = useSystemStore(s => s.sab);

  const bodyRef = useRef<THREE.InstancedMesh>(null);
  const canopyRef = useRef<THREE.InstancedMesh>(null);
  const armRef = useRef<THREE.InstancedMesh>(null);
  const motorRef = useRef<THREE.InstancedMesh>(null);
  const propRef = useRef<THREE.InstancedMesh>(null);
  const ledRef = useRef<THREE.InstancedMesh>(null);

  const flagsRef = useRef<Int32Array | null>(null);
  const stateViewRef = useRef<Float32Array | null>(null);
  const lastEpochRef = useRef<number>(-1);

  const stateCacheRef = useRef({
    pos: new Float32Array(DRONE_CONSTANTS.MAX_DRONES * 3),
    quat: new Float32Array(DRONE_CONSTANTS.MAX_DRONES * 4),
    rpm: new Float32Array(DRONE_CONSTANTS.MAX_DRONES * 4),
  });

  const armOffsets = useMemo(
    () =>
      ARM_ANGLES.map(angle => ({
        x: Math.cos(angle) * ARM_LENGTH,
        z: Math.sin(angle) * ARM_LENGTH,
      })),
    []
  );

  const geometry = useMemo(() => {
    const body = new THREE.BoxGeometry(0.22, 0.05, 0.22);
    const canopy = new THREE.CapsuleGeometry(0.08, 0.12, 4, 8);
    const arm = new THREE.CylinderGeometry(0.01, 0.012, ARM_LENGTH, 10);
    const motor = new THREE.CylinderGeometry(MOTOR_RADIUS, MOTOR_RADIUS * 0.9, 0.03, 16);
    const prop = new THREE.CircleGeometry(PROP_RADIUS, 20);
    const led = new THREE.SphereGeometry(0.015, 10, 10);

    arm.rotateZ(Math.PI / 2);
    prop.rotateX(-Math.PI / 2);

    return { body, canopy, arm, motor, prop, led };
  }, []);

  const materials = useMemo(
    () => ({
      body: new THREE.MeshPhysicalMaterial({
        color: '#0f172a',
        roughness: 0.35,
        metalness: 0.65,
        clearcoat: 0.3,
        clearcoatRoughness: 0.2,
        vertexColors: true,
      }),
      canopy: new THREE.MeshStandardMaterial({
        color: '#111827',
        roughness: 0.4,
        metalness: 0.5,
      }),
      arm: new THREE.MeshStandardMaterial({
        color: '#374151',
        roughness: 0.6,
        metalness: 0.35,
      }),
      motor: new THREE.MeshStandardMaterial({
        color: '#1f2937',
        roughness: 0.4,
        metalness: 0.8,
      }),
      prop: new THREE.MeshBasicMaterial({
        color: '#cbd5f5',
        transparent: true,
        opacity: 0.45,
        depthWrite: false,
        side: THREE.DoubleSide,
      }),
      led: new THREE.MeshBasicMaterial({
        color: '#ffffff',
        transparent: true,
        opacity: 0.9,
        toneMapped: false,
        vertexColors: true,
      }),
    }),
    []
  );

  useEffect(() => {
    if (!bodyRef.current || !ledRef.current) return;

    for (let i = 0; i < DRONE_CONSTANTS.MAX_DRONES; i++) {
      tempColor.set(DRONE_COLORS[i % DRONE_COLORS.length]);
      bodyRef.current.setColorAt(i, tempColor);

      const ledBase = i * 4;
      ledRef.current.setColorAt(ledBase, tempColor);
      ledRef.current.setColorAt(ledBase + 1, tempColor);
      tempColor.set('#22d3ee');
      ledRef.current.setColorAt(ledBase + 2, tempColor);
      ledRef.current.setColorAt(ledBase + 3, tempColor);
    }

    bodyRef.current.instanceColor!.needsUpdate = true;
    ledRef.current.instanceColor!.needsUpdate = true;
  }, []);

  useFrame(state => {
    if (!sab) return;

    if (!flagsRef.current || flagsRef.current.buffer !== sab) {
      flagsRef.current = new Int32Array(sab, 0, 256);
    }

    const flags = flagsRef.current;
    const epoch = Atomics.load(flags, EPOCH_INDICES.DRONE_PHYSICS);
    const strideFloats = DRONE_CONSTANTS.STRIDE / 4;
    const totalFloats = DRONE_CONSTANTS.MAX_DRONES * strideFloats;
    const isBufferA = epoch % 2 === 0;
    const offset = isBufferA ? SAB_OFFSETS.DRONE_STATE_A : SAB_OFFSETS.DRONE_STATE_B;

    if (
      !stateViewRef.current ||
      stateViewRef.current.buffer !== sab ||
      stateViewRef.current.byteOffset !== offset
    ) {
      stateViewRef.current = new Float32Array(sab, offset, totalFloats);
    }

    const stateView = stateViewRef.current;
    const caches = stateCacheRef.current;

    if (epoch !== lastEpochRef.current) {
      lastEpochRef.current = epoch;

      for (let i = 0; i < DRONE_CONSTANTS.MAX_DRONES; i++) {
        const base = i * strideFloats;
        const pBase = i * 3;
        const qBase = i * 4;
        const rBase = i * 4;

        caches.pos[pBase] = stateView[base + 0];
        caches.pos[pBase + 1] = stateView[base + 1];
        caches.pos[pBase + 2] = stateView[base + 2];

        caches.quat[qBase] = stateView[base + 8];
        caches.quat[qBase + 1] = stateView[base + 9];
        caches.quat[qBase + 2] = stateView[base + 10];
        caches.quat[qBase + 3] = stateView[base + 11];

        caches.rpm[rBase] = stateView[base + 16];
        caches.rpm[rBase + 1] = stateView[base + 17];
        caches.rpm[rBase + 2] = stateView[base + 18];
        caches.rpm[rBase + 3] = stateView[base + 19];

        const px = caches.pos[pBase];
        const py = caches.pos[pBase + 1];
        const pz = caches.pos[pBase + 2];
        const qw = caches.quat[qBase];
        const qx = caches.quat[qBase + 1];
        const qy = caches.quat[qBase + 2];
        const qz = caches.quat[qBase + 3];

        tempQuat.set(qx, qy, qz, qw);

        if (bodyRef.current) {
          dummyObject.position.set(px, py, pz);
          dummyObject.quaternion.copy(tempQuat);
          dummyObject.scale.set(1, 1, 1);
          dummyObject.updateMatrix();
          bodyRef.current.setMatrixAt(i, dummyObject.matrix);
        }

        if (canopyRef.current) {
          dummyObject.position.set(px, py + 0.03, pz + 0.03);
          dummyObject.quaternion.copy(tempQuat);
          dummyObject.scale.set(1, 1, 1);
          dummyObject.updateMatrix();
          canopyRef.current.setMatrixAt(i, dummyObject.matrix);
        }

        if (armRef.current || motorRef.current || ledRef.current) {
          for (let a = 0; a < 4; a++) {
            const armIndex = i * 4 + a;
            const armOffset = armOffsets[a];

            tempVec.set(armOffset.x / 2, 0, armOffset.z / 2);
            tempVec.applyQuaternion(tempQuat);

            if (armRef.current) {
              dummyObject.position.set(px + tempVec.x, py, pz + tempVec.z);
              dummyObject.quaternion.copy(tempQuat);
              dummyObject.rotateY(ARM_ANGLES[a]);
              dummyObject.scale.set(1, 1, 1);
              dummyObject.updateMatrix();
              armRef.current.setMatrixAt(armIndex, dummyObject.matrix);
            }

            tempVec.set(armOffset.x, 0.015, armOffset.z);
            tempVec.applyQuaternion(tempQuat);

            if (motorRef.current) {
              dummyObject.position.set(px + tempVec.x, py + tempVec.y, pz + tempVec.z);
              dummyObject.quaternion.copy(tempQuat);
              dummyObject.scale.set(1, 1, 1);
              dummyObject.updateMatrix();
              motorRef.current.setMatrixAt(armIndex, dummyObject.matrix);
            }

            if (ledRef.current) {
              const ledIndex = i * 4 + a;
              dummyObject.position.set(px + tempVec.x, py + tempVec.y + 0.02, pz + tempVec.z);
              dummyObject.quaternion.copy(tempQuat);
              dummyObject.scale.set(1, 1, 1);
              dummyObject.updateMatrix();
              ledRef.current.setMatrixAt(ledIndex, dummyObject.matrix);
            }
          }
        }
      }

      if (bodyRef.current) bodyRef.current.instanceMatrix.needsUpdate = true;
      if (canopyRef.current) canopyRef.current.instanceMatrix.needsUpdate = true;
      if (armRef.current) armRef.current.instanceMatrix.needsUpdate = true;
      if (motorRef.current) motorRef.current.instanceMatrix.needsUpdate = true;
      if (ledRef.current) ledRef.current.instanceMatrix.needsUpdate = true;
    }

    if (propRef.current) {
      const time = state.clock.elapsedTime;

      for (let i = 0; i < DRONE_CONSTANTS.MAX_DRONES; i++) {
        const pBase = i * 3;
        const qBase = i * 4;
        const rBase = i * 4;

        const px = caches.pos[pBase];
        const py = caches.pos[pBase + 1];
        const pz = caches.pos[pBase + 2];
        const qw = caches.quat[qBase];
        const qx = caches.quat[qBase + 1];
        const qy = caches.quat[qBase + 2];
        const qz = caches.quat[qBase + 3];

        tempQuat.set(qx, qy, qz, qw);

        for (let a = 0; a < 4; a++) {
          const armIndex = i * 4 + a;
          const armOffset = armOffsets[a];
          const rpm = caches.rpm[rBase + a] || 12000;
          const spin = time * rpm * RPM_TO_RAD;
          const spinDirection = MOTOR_DIR[a] ?? 1;
          const blurScale = 0.55 + (rpm / MAX_MOTOR_RPM) * 0.35;

          tempVec.set(armOffset.x, 0.025, armOffset.z);
          tempVec.applyQuaternion(tempQuat);

          dummyObject.position.set(px + tempVec.x, py + tempVec.y, pz + tempVec.z);
          dummyObject.quaternion.copy(tempQuat);
          dummyObject.rotateY(spin * spinDirection);
          dummyObject.scale.set(blurScale, 1, blurScale);
          dummyObject.updateMatrix();

          propRef.current.setMatrixAt(armIndex, dummyObject.matrix);
        }
      }

      propRef.current.instanceMatrix.needsUpdate = true;
    }
  });

  return (
    <group>
      <instancedMesh
        ref={bodyRef}
        args={[geometry.body, materials.body, DRONE_CONSTANTS.MAX_DRONES]}
        frustumCulled={false}
        castShadow
      />
      <instancedMesh
        ref={canopyRef}
        args={[geometry.canopy, materials.canopy, DRONE_CONSTANTS.MAX_DRONES]}
        frustumCulled={false}
        castShadow
      />
      <instancedMesh
        ref={armRef}
        args={[geometry.arm, materials.arm, DRONE_CONSTANTS.MAX_DRONES * 4]}
        frustumCulled={false}
        castShadow
      />
      <instancedMesh
        ref={motorRef}
        args={[geometry.motor, materials.motor, DRONE_CONSTANTS.MAX_DRONES * 4]}
        frustumCulled={false}
        castShadow
      />
      <instancedMesh
        ref={propRef}
        args={[geometry.prop, materials.prop, DRONE_CONSTANTS.MAX_DRONES * 4]}
        frustumCulled={false}
      />
      <instancedMesh
        ref={ledRef}
        args={[geometry.led, materials.led, DRONE_CONSTANTS.MAX_DRONES * 4]}
        frustumCulled={false}
      />
    </group>
  );
}
