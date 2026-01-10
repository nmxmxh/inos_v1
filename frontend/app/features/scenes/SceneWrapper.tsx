/**
 * INOS Graphics Scenes — SceneWrapper
 *
 * Architecture-aligned with graphics.md principles:
 * - Sync background camera with boids simulation
 * - Fade-in transitions for seamless graphics entry
 * - Cached TypedArray views (no per-frame allocation)
 * - Context versioning (zombie killing)
 * - SAB arena offset management
 * - Fullscreen toggle, FPS counter
 */

import { useRef, useState, useEffect, ReactNode, Suspense, useMemo } from 'react';
import styled from 'styled-components';
import { Canvas } from '@react-three/fiber';
import { EffectComposer, Bloom, Noise, Vignette } from '@react-three/postprocessing';

// ========== SAB ARENA ALLOCATION ==========
// Free space starts after Matrix Buffer B at 0x01022000
// ~15MB available in 32MB Light tier

const ARENA_SCENE_BASE = 0x01022000;

export const SCENE_OFFSETS = {
  // Terrain: 100x100 heightmap * 4 bytes = 40KB
  TERRAIN_HEIGHTMAP: ARENA_SCENE_BASE,
  TERRAIN_SIZE: 100 * 100 * 4,

  // Clouds: 64x64 density map * 4 bytes = 16KB
  CLOUD_MAP: ARENA_SCENE_BASE + 100 * 100 * 4,
  CLOUD_SIZE: 64 * 64 * 4,

  // Boids offsets are defined in sab_layout.capnp and exported from layout.ts
};

// ========== CACHED VIEW MANAGEMENT ==========
// Per graphics.md: "All TypedArray views are cached at initialization"

const viewCache = new Map<string, Float32Array>();

export function getArenaView(sab: SharedArrayBuffer, offset: number, size: number): Float32Array {
  const key = `${offset}:${size}`;
  let view = viewCache.get(key);

  if (!view || view.buffer !== sab) {
    // Create and cache new view only if SAB changed
    view = new Float32Array(sab, offset, size / 4);
    viewCache.set(key, view);
  }

  return view;
}

// Clear cache on context change
export function clearViewCache() {
  viewCache.clear();
}

// ========== STYLED COMPONENTS ==========

const Container = styled.div<{ $isFullscreen: boolean; $isBackground?: boolean }>`
  position: ${p => (p.$isBackground ? 'fixed' : 'relative')};
  top: ${p => (p.$isBackground ? '0' : 'auto')};
  left: ${p => (p.$isBackground ? '0' : 'auto')};
  width: 100%;
  height: ${p => (p.$isBackground ? '100vh' : p.$isFullscreen ? '100vh' : '400px')};
  background: ${p =>
    p.$isBackground ? 'transparent' : 'linear-gradient(135deg, #0f172a 0%, #1e293b 100%)'};
  border-radius: ${p => (p.$isFullscreen || p.$isBackground ? '0' : '8px')};
  overflow: hidden;
  transition:
    height 0.3s ease,
    opacity 1s ease;
  z-index: ${p => (p.$isBackground ? -2 : 'auto')};
  pointer-events: ${p => (p.$isBackground ? 'none' : 'auto')};
  opacity: ${p => (p.$isBackground ? 1 : 1)}; // Placeholder for future fade
`;

const Header = styled.div`
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: linear-gradient(to bottom, rgba(0, 0, 0, 0.6), transparent);
  z-index: 10;
  pointer-events: none;

  > * {
    pointer-events: auto;
  }
`;

const Title = styled.div`
  color: white;
  font-size: 14px;
  font-weight: 600;
  text-shadow: 0 1px 2px rgba(0, 0, 0, 0.5);
`;

const Controls = styled.div`
  display: flex;
  gap: 8px;
  align-items: center;
`;

const ControlButton = styled.button`
  width: 32px;
  height: 32px;
  border-radius: 6px;
  border: 1px solid rgba(255, 255, 255, 0.2);
  background: rgba(255, 255, 255, 0.1);
  color: white;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
  transition: all 0.2s ease;

  &:hover {
    background: rgba(255, 255, 255, 0.2);
    border-color: rgba(255, 255, 255, 0.4);
  }
`;

const FPSCounter = styled.div`
  font-family: 'JetBrains Mono', monospace;
  font-size: 11px;
  color: #10b981;
  background: rgba(0, 0, 0, 0.5);
  padding: 4px 8px;
  border-radius: 4px;
`;

const LoadingFallback = styled.div`
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: rgba(255, 255, 255, 0.6);

  .spinner {
    width: 40px;
    height: 40px;
    border: 3px solid rgba(255, 255, 255, 0.2);
    border-top-color: #8b5cf6;
    border-radius: 50%;
    animation: spin 1s linear infinite;
    margin-bottom: 12px;
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
`;

const SABIndicator = styled.div<{ $connected: boolean }>`
  font-size: 10px;
  padding: 4px 8px;
  border-radius: 4px;
  background: ${p => (p.$connected ? 'rgba(16, 185, 129, 0.2)' : 'rgba(239, 68, 68, 0.2)')};
  color: ${p => (p.$connected ? '#10b981' : '#ef4444')};
  border: 1px solid ${p => (p.$connected ? 'rgba(16, 185, 129, 0.3)' : 'rgba(239, 68, 68, 0.3)')};
`;

// ========== FPS HOOK (optimized, no state in loop) ==========

function useFPS() {
  const [fps, setFps] = useState(60);
  const frameCount = useRef(0);
  const lastTime = useRef(performance.now());

  useEffect(() => {
    let animId: number;
    const contextId = (window as any).__INOS_CONTEXT_ID__;

    const loop = () => {
      // Zombie killing: exit if context changed
      if ((window as any).__INOS_CONTEXT_ID__ !== contextId) return;

      frameCount.current++;
      const now = performance.now();
      const delta = now - lastTime.current;

      if (delta >= 1000) {
        setFps(Math.round((frameCount.current * 1000) / delta));
        frameCount.current = 0;
        lastTime.current = now;
      }

      animId = requestAnimationFrame(loop);
    };

    animId = requestAnimationFrame(loop);
    return () => cancelAnimationFrame(animId);
  }, []);

  return fps;
}

// ========== SAB HOOK (event-based, not polling) ==========

export function useSAB(): SharedArrayBuffer | null {
  const [sab, setSab] = useState<SharedArrayBuffer | null>(() => {
    // Immediate check on mount
    return (window as any).__INOS_SAB__ || null;
  });

  useEffect(() => {
    // Check once more after mount (kernel may have initialized)
    const globalSab = (window as any).__INOS_SAB__;
    if (globalSab && globalSab !== sab) {
      setSab(globalSab);
    }

    // Listen for custom SAB ready event (from kernel.ts)
    const handleSABReady = () => {
      const newSab = (window as any).__INOS_SAB__;
      if (newSab) setSab(newSab);
    };

    window.addEventListener('inos:sab-ready', handleSABReady);
    return () => window.removeEventListener('inos:sab-ready', handleSABReady);
  }, [sab]);

  return sab;
}

// ========== SCENE WRAPPER ==========

interface SceneWrapperProps {
  title: string;
  children: ReactNode;
  showFPS?: boolean;
  showSABStatus?: boolean;
  isBackground?: boolean;
}

export default function SceneWrapper({
  title,
  children,
  showFPS = true,
  showSABStatus = true,
  isBackground = false,
}: SceneWrapperProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const fps = useFPS();
  const sab = useSAB();

  // Context versioning
  const contextId = useMemo(() => (window as any).__INOS_CONTEXT_ID__ || 0, []);

  useEffect(() => {
    const handleFullscreenChange = () => {
      setIsFullscreen(!!document.fullscreenElement);
    };

    document.addEventListener('fullscreenchange', handleFullscreenChange);
    return () => document.removeEventListener('fullscreenchange', handleFullscreenChange);
  }, []);

  const toggleFullscreen = () => {
    if (!document.fullscreenElement) {
      containerRef.current?.requestFullscreen();
    } else {
      document.exitFullscreen();
    }
  };

  return (
    <Container
      ref={containerRef}
      $isFullscreen={isFullscreen}
      $isBackground={isBackground}
      data-context-id={contextId}
    >
      {!isBackground && (
        <Header>
          <Title>{title}</Title>
          <Controls>
            {showSABStatus && (
              <SABIndicator $connected={!!sab}>{sab ? 'SAB Connected' : 'Demo Mode'}</SABIndicator>
            )}
            {showFPS && <FPSCounter>{fps} FPS</FPSCounter>}
            <ControlButton onClick={toggleFullscreen} title="Toggle Fullscreen">
              ⛶
            </ControlButton>
          </Controls>
        </Header>
      )}

      <Suspense
        fallback={
          <LoadingFallback>
            <div className="spinner" />
            <span>Loading scene...</span>
          </LoadingFallback>
        }
      >
        {isBackground && (
          <>
            <div
              style={{
                position: 'absolute',
                top: 0,
                left: 0,
                right: 0,
                height: '100px',
                background: 'linear-gradient(to bottom, rgba(244, 241, 234, 0.4), transparent)',
                zIndex: 1,
              }}
            />
            <div
              style={{
                position: 'absolute',
                bottom: 0,
                left: 0,
                right: 0,
                height: '80px',
                background: 'linear-gradient(to top, rgba(244, 241, 234, 0.4), transparent)',
                zIndex: 1,
              }}
            />
          </>
        )}
        <Canvas
          dpr={[1, 2]}
          camera={
            isBackground ? { position: [0, 10, 40], fov: 45 } : { position: [0, 5, 15], fov: 50 }
          }
          gl={{
            alpha: true,
            antialias: false, // Postprocessing handles AA better
            powerPreference: 'high-performance',
            stencil: false,
            depth: true,
          }}
        >
          {children}

          <EffectComposer enableNormalPass={false}>
            <Bloom
              luminanceThreshold={0.4}
              mipmapBlur
              intensity={isBackground ? 0.5 : 0.8}
              radius={0.4}
            />
            <Noise opacity={isBackground ? 0.02 : 0.05} />
            <Vignette eskil={false} offset={0.1} darkness={1.1} />
          </EffectComposer>
        </Canvas>
      </Suspense>
    </Container>
  );
}
