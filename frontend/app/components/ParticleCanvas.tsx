import { useRef, useEffect } from 'react';

interface Particle {
  x: number;
  y: number;
  vx: number;
  vy: number;
  mass: number;
  module: 'ml' | 'science' | 'compute';
}

const PARTICLE_COUNT = 150;
const G = 5.0; // MUCH stronger gravity for dramatic movement
const SOFTENING = 15;
const DAMPING = 1.0; // NO damping - conserve energy!

export default function ParticleCanvas() {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const particlesRef = useRef<Particle[]>([]);
  const animationRef = useRef<number>();
  const frameCountRef = useRef(0);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) {
      console.warn('[ParticleCanvas] Canvas ref not available');
      return;
    }

    const ctx = canvas.getContext('2d');
    if (!ctx) {
      console.error('[ParticleCanvas] Failed to get 2D context');
      return;
    }

    console.log('[ParticleCanvas] ✅ Initializing particle system');

    // Set canvas size
    const resize = () => {
      const dpr = window.devicePixelRatio || 1;
      canvas.width = window.innerWidth * dpr;
      canvas.height = window.innerHeight * dpr;
      canvas.style.width = `${window.innerWidth}px`;
      canvas.style.height = `${window.innerHeight}px`;
      ctx.scale(dpr, dpr);
      console.log(
        `[ParticleCanvas] 📐 Resized to ${window.innerWidth}x${window.innerHeight}, DPR: ${dpr}`
      );
    };
    resize();
    window.addEventListener('resize', resize);

    // Initialize particles
    const initParticles = () => {
      const particles: Particle[] = [];
      const modules: Array<'ml' | 'science' | 'compute'> = ['ml', 'science', 'compute'];
      const centerX = window.innerWidth / 2;
      const centerY = window.innerHeight / 2;

      for (let i = 0; i < PARTICLE_COUNT; i++) {
        const angle = (i / PARTICLE_COUNT) * Math.PI * 2;
        const radius = 200 + Math.random() * 150;
        const module = modules[i % 3];

        particles.push({
          x: centerX + Math.cos(angle) * radius,
          y: centerY + Math.sin(angle) * radius,
          vx: Math.sin(angle) * 5.0, // Much faster initial velocity
          vy: -Math.cos(angle) * 5.0,
          mass: 1.5 + Math.random() * 1.5,
          module,
        });
      }

      particlesRef.current = particles;
      console.log(`[ParticleCanvas] 🎯 Initialized ${particles.length} particles`);
      console.log(`[ParticleCanvas] 📊 Sample particle:`, {
        x: particles[0].x.toFixed(2),
        y: particles[0].y.toFixed(2),
        vx: particles[0].vx.toFixed(2),
        vy: particles[0].vy.toFixed(2),
        mass: particles[0].mass.toFixed(2),
      });
    };

    initParticles();

    // N-body simulation
    const updateParticles = () => {
      const particles = particlesRef.current;
      const dt = 0.016;

      for (let i = 0; i < particles.length; i++) {
        const p1 = particles[i];
        let fx = 0;
        let fy = 0;

        // Calculate gravitational forces
        for (let j = 0; j < particles.length; j++) {
          if (i === j) continue;

          const p2 = particles[j];
          const dx = p2.x - p1.x;
          const dy = p2.y - p1.y;
          const distSq = dx * dx + dy * dy + SOFTENING * SOFTENING;
          const invDist = 1 / Math.sqrt(distSq);
          const invDistCube = invDist * invDist * invDist;

          const force = G * p1.mass * p2.mass * invDistCube;
          fx += dx * force;
          fy += dy * force;
        }

        // Update velocity
        const ax = fx / p1.mass;
        const ay = fy / p1.mass;
        p1.vx += ax * dt;
        p1.vy += ay * dt;
        p1.vx *= DAMPING;
        p1.vy *= DAMPING;

        // Add random energy injection to prevent equilibrium
        if (i % 10 === 0) {
          p1.vx += (Math.random() - 0.5) * 0.5;
          p1.vy += (Math.random() - 0.5) * 0.5;
        }

        // Update position
        p1.x += p1.vx * dt;
        p1.y += p1.vy * dt;

        // Wrap around edges
        if (p1.x < 0) p1.x = window.innerWidth;
        if (p1.x > window.innerWidth) p1.x = 0;
        if (p1.y < 0) p1.y = window.innerHeight;
        if (p1.y > window.innerHeight) p1.y = 0;
      }
    };

    // Spider-Verse style rendering
    const render = () => {
      const particles = particlesRef.current;

      // Clear canvas completely - no trails (they were covering particles)
      ctx.fillStyle = '#ffffff';
      ctx.fillRect(0, 0, window.innerWidth, window.innerHeight);

      // Glitch effect (random chromatic aberration)
      if (Math.random() < 0.05) {
        ctx.save();
        ctx.globalCompositeOperation = 'screen';
        ctx.translate(Math.random() * 4 - 2, Math.random() * 4 - 2);
      }

      // Draw connection lines (black and white)
      ctx.strokeStyle = 'rgba(0, 0, 0, 0.1)';
      ctx.lineWidth = 1;
      for (let i = 0; i < particles.length; i++) {
        const p1 = particles[i];
        for (let j = i + 1; j < particles.length; j++) {
          const p2 = particles[j];
          const dx = p2.x - p1.x;
          const dy = p2.y - p1.y;
          const dist = Math.sqrt(dx * dx + dy * dy);

          if (dist < 150) {
            const alpha = (1 - dist / 150) * 0.15;
            ctx.strokeStyle = `rgba(0, 0, 0, ${alpha})`;
            ctx.beginPath();
            ctx.moveTo(p1.x, p1.y);
            ctx.lineTo(p2.x, p2.y);
            ctx.stroke();
          }
        }
      }

      // Draw particles (black and white with halftone effect)
      for (const particle of particles) {
        const radius = particle.mass * 8;

        // Halftone dots pattern
        const dotSize = radius * 0.3;
        const dotSpacing = radius * 0.5;
        for (let dx = -radius; dx <= radius; dx += dotSpacing) {
          for (let dy = -radius; dy <= radius; dy += dotSpacing) {
            const dist = Math.sqrt(dx * dx + dy * dy);
            if (dist < radius) {
              const intensity = 1 - dist / radius;
              ctx.fillStyle = `rgba(0, 0, 0, ${intensity * 0.8})`;
              ctx.beginPath();
              ctx.arc(particle.x + dx, particle.y + dy, dotSize, 0, Math.PI * 2);
              ctx.fill();
            }
          }
        }

        // Main particle (solid black)
        ctx.fillStyle = '#000000';
        ctx.beginPath();
        ctx.arc(particle.x, particle.y, radius, 0, Math.PI * 2);
        ctx.fill();

        // White outline (comic book style)
        ctx.strokeStyle = '#ffffff';
        ctx.lineWidth = 2;
        ctx.beginPath();
        ctx.arc(particle.x, particle.y, radius, 0, Math.PI * 2);
        ctx.stroke();

        // Glitch lines (Spider-Verse style)
        if (Math.random() < 0.1) {
          ctx.strokeStyle = `rgba(0, 0, 0, ${Math.random() * 0.5})`;
          ctx.lineWidth = 1;
          ctx.beginPath();
          ctx.moveTo(particle.x - radius * 2, particle.y);
          ctx.lineTo(particle.x + radius * 2, particle.y);
          ctx.stroke();
        }
      }

      // Restore after glitch
      if (Math.random() < 0.05) {
        ctx.restore();
      }

      // Scanlines effect
      ctx.fillStyle = 'rgba(0, 0, 0, 0.03)';
      for (let y = 0; y < window.innerHeight; y += 4) {
        ctx.fillRect(0, y, window.innerWidth, 2);
      }
    };

    // Animation loop with detailed logging
    const animate = () => {
      const frame = frameCountRef.current++;

      updateParticles();
      render();

      // Log every 60 frames (1 second)
      if (frame % 60 === 0) {
        const p = particlesRef.current[0];
        console.log(`[ParticleCanvas] 🎬 Frame ${frame}:`, {
          particles: particlesRef.current.length,
          sample_pos: `(${p.x.toFixed(1)}, ${p.y.toFixed(1)})`,
          sample_vel: `(${p.vx.toFixed(3)}, ${p.vy.toFixed(3)})`,
          speed: Math.sqrt(p.vx * p.vx + p.vy * p.vy).toFixed(3),
        });
      }

      // Log particle buffer every 5 seconds
      if (frame % 300 === 0 && frame > 0) {
        console.log('[ParticleCanvas] 📦 Particle Buffer Sample (first 3):');
        particlesRef.current.slice(0, 3).forEach((p, i) => {
          console.log(`  [${i}]`, {
            pos: `(${p.x.toFixed(1)}, ${p.y.toFixed(1)})`,
            vel: `(${p.vx.toFixed(3)}, ${p.vy.toFixed(3)})`,
            mass: p.mass.toFixed(2),
            module: p.module,
          });
        });
      }

      animationRef.current = requestAnimationFrame(animate);
    };

    console.log('[ParticleCanvas] 🚀 Starting animation loop');
    animate();

    return () => {
      console.log('[ParticleCanvas] 🛑 Stopping animation');
      window.removeEventListener('resize', resize);
      if (animationRef.current) {
        cancelAnimationFrame(animationRef.current);
      }
    };
  }, []);

  return (
    <div
      style={{
        position: 'fixed',
        top: 0,
        left: 0,
        width: '100vw',
        height: '100vh',
        zIndex: 0,
        pointerEvents: 'none',
        background: '#ffffff',
      }}
    >
      <canvas
        ref={canvasRef}
        style={{
          display: 'block',
          width: '100%',
          height: '100%',
          imageRendering: 'crisp-edges', // Pixelated effect
        }}
      />

      {/* Legend - Spider-Verse style */}
      <div
        style={{
          position: 'absolute',
          bottom: '100px',
          right: '40px',
          display: 'flex',
          flexDirection: 'column',
          gap: '12px',
          fontFamily: 'monospace',
          fontSize: '11px',
          color: '#000',
          pointerEvents: 'auto',
          background: '#fff',
          padding: '16px',
          border: '2px solid #000',
          boxShadow: '4px 4px 0 #000',
        }}
      >
        <div style={{ fontWeight: 'bold', marginBottom: '8px' }}>N-BODY MESH</div>
        <div>
          {PARTICLE_COUNT} particles • G={G} • Damping={DAMPING}
        </div>
        <div style={{ fontSize: '10px', color: '#666', marginTop: '8px' }}>
          Spider-Verse rendering mode
        </div>
      </div>
    </div>
  );
}
