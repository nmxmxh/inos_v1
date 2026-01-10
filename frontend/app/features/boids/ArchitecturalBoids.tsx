import { Canvas } from '@react-three/fiber';
import { Suspense } from 'react';
import { useLocation } from 'react-router-dom';
import { AnimatePresence, motion } from 'framer-motion';
import InstancedBoidsRenderer from './InstancedBoidsRenderer';

export default function ArchitecturalBoids() {
  const location = useLocation();
  const isGraphicsPage = location.pathname.includes('/graphics');

  return (
    <AnimatePresence>
      {!isGraphicsPage && (
        <motion.div
          key="architectural-boids"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 1 }}
          style={{
            width: '100%',
            height: '100%',
            position: 'fixed',
            top: 0,
            left: 0,
            zIndex: -1,
            pointerEvents: 'none',
            background: '#f4f1ea', // --paper-cream
          }}
        >
          {/* Noise Overlay - Integrated directly into boids layer */}
          <div
            style={{
              position: 'absolute',
              top: 0,
              left: 0,
              width: '100%',
              height: '100%',
              opacity: 0.15,
              pointerEvents: 'none',
              backgroundImage: `url("data:image/svg+xml,%3Csvg viewBox='0 0 200 200' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='noiseFilter'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.8' numOctaves='4' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23noiseFilter)' opacity='0.02'/%3E%3C/svg%3E")`,
              zIndex: 1,
            }}
          />

          <Suspense fallback={null}>
            <Canvas
              camera={{ position: [0, 8, 30], fov: 45 }}
              style={{ background: 'transparent' }}
              dpr={[1, 2]}
              gl={{
                alpha: true,
                antialias: true,
                powerPreference: 'high-performance',
              }}
            >
              <ambientLight intensity={0.6} />
              <pointLight position={[15, 15, 15]} intensity={0.8} />
              <InstancedBoidsRenderer />
              <gridHelper args={[80, 10, '#330066', '#110022']} position={[0, -10, 0]} />
            </Canvas>
          </Suspense>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
