import { Canvas } from '@react-three/fiber';
import { Suspense } from 'react';
import { useLocation } from 'react-router-dom';
import { AnimatePresence, motion } from 'framer-motion';

import { TerrainScene } from '../scenes';
import InstancedBoidsRenderer from './InstancedBoidsRenderer';

export default function ArchitecturalBoids() {
  const location = useLocation();
  const isGraphicsPage = location.pathname.includes('/graphics');

  return (
    <AnimatePresence mode="wait">
      {isGraphicsPage ? (
        <motion.div
          key="terrain-scene"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 1 }}
          style={{
            position: 'fixed',
            top: 0,
            left: 0,
            width: '100vw',
            height: '100vh',
            zIndex: -1,
            pointerEvents: 'none',
          }}
        >
          <TerrainScene isBackground={true} />
        </motion.div>
      ) : (
        <motion.div
          key="architectural-boids"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 1 }}
          style={{
            width: '100vw',
            height: '100vh',
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
              opacity: 0.06,
              pointerEvents: 'none',
              backgroundImage: `url("data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAADAAAAAwBAMAAAClLOS0AAAAElBMVEUAAAD8/vz08teleO7m6NzJybt+JT4mAAAACXBIWXMAAAsTAAALEwEAmpwYAAAAB3RJTUUH4QkUBwov0mxiMwAAAB1JREFUOMUD2AEJxDAMBMAfXR5YCZAwIEAAC7sP8H+NZmZ2xq8AAAAASUVORK5CYII=")`,
              zIndex: 1,
            }}
          />

          <Suspense fallback={null}>
            <Canvas
              camera={{ position: [15, 12, 45], fov: 42 }}
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
