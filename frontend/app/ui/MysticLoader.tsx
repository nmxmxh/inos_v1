/**
 * INOS Technical Codex — History Loader
 *
 * "The Computation Journey"
 * A minimal, abstract timeline of computing history:
 * 1. ORIGINS (Abacus/Beads) - Calculation
 * 2. LOGIC (Punch Card/Binary) - Programming
 * 3. SILICON (Circuit/Chip) - Integrated Processing
 * 4. ETHER (Mesh/Network) - Distributed Intelligence (INOS)
 */

import { motion, AnimatePresence } from 'framer-motion';
import { useState, useEffect } from 'react';
import { theme } from '../styles/theme';
import { SystemStatus } from '../../src/store/system';

interface MysticLoaderProps {
  status: SystemStatus;
}

// ═══════════════════════════════════════════════════════════════════
// ERAS
// ═══════════════════════════════════════════════════════════════════

type Era = 'origins' | 'logic' | 'silicon' | 'ether';

const ERA_SEQUENCE: Era[] = ['origins', 'logic', 'silicon', 'ether'];
const ERA_TITLES = {
  origins: 'CALCULATION',
  logic: 'PROGRAMMING',
  silicon: 'INTEGRATION',
  ether: 'DISTRIBUTION',
};

// ═══════════════════════════════════════════════════════════════════
// COMPONENTS
// ═══════════════════════════════════════════════════════════════════

const CenteredFrame = ({ children }: { children: React.ReactNode }) => (
  <div
    style={{
      width: 300,
      height: 300,
      position: 'relative',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
    }}
  >
    {children}
  </div>
);

// ERA 1: ABACUS (Beads on lines)
const AbacusEra = () => (
  <svg width="200" height="200" viewBox="0 0 200 200">
    {[50, 100, 150].map((y, i) => (
      <g key={i}>
        {/* Rod */}
        <line x1="20" y1={y} x2="180" y2={y} stroke={theme.colors.inkLight} strokeWidth="1" />
        {/* Beads */}
        {[0, 1, 2].map(b => (
          <motion.circle
            key={b}
            cx={40 + b * 20}
            cy={y}
            r="6"
            fill="none"
            stroke={theme.colors.inkDark}
            strokeWidth="2"
            initial={{ cx: 40 + b * 20 }}
            animate={{ cx: [40 + b * 20, 140 + b * 20, 40 + b * 20] }}
            transition={{
              duration: 2,
              ease: 'easeInOut',
              delay: i * 0.2 + b * 0.1,
              repeat: Infinity,
              repeatDelay: 0.5,
            }}
          />
        ))}
      </g>
    ))}
  </svg>
);

// ERA 2: PUNCH CARD (Grid of binary states)
const LogicEra = () => {
  const customGrid = [
    [1, 0, 1, 1, 0, 1],
    [0, 1, 0, 1, 1, 0],
    [1, 1, 1, 0, 0, 1],
    [0, 0, 1, 1, 0, 0],
    [1, 0, 0, 0, 1, 1],
    [0, 1, 1, 1, 0, 1],
  ];

  return (
    <svg width="200" height="200" viewBox="0 0 200 200">
      <rect
        x="10"
        y="10"
        width="180"
        height="180"
        fill="none"
        stroke={theme.colors.inkLight}
        strokeWidth="1"
        rx="4"
      />
      {customGrid.map((row, r) => (
        <g key={r} transform={`translate(0, ${r * 30 + 20})`}>
          {row.map((val, c) => (
            <motion.rect
              key={c}
              x={c * 30 + 17.5}
              y="0"
              width="15"
              height="10"
              fill={val ? theme.colors.inkDark : 'none'}
              stroke={theme.colors.inkDark}
              strokeWidth="1"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              transition={{ delay: r * 0.1 + c * 0.05 }}
            />
          ))}
        </g>
      ))}
      {/* Reader Head Line */}
      <motion.line
        x1="10"
        y1="20"
        x2="190"
        y2="20"
        stroke={theme.colors.accent}
        strokeWidth="2"
        animate={{ y: [20, 180, 20] }}
        transition={{ duration: 1.5, ease: 'linear', repeat: Infinity }}
        opacity={0.5}
      />
    </svg>
  );
};

// ERA 3: SILICON (Chip & Traces)
const SiliconEra = () => (
  <svg width="200" height="200" viewBox="0 0 200 200">
    {/* Microchip Body */}
    <rect
      x="60"
      y="60"
      width="80"
      height="80"
      fill="none"
      stroke={theme.colors.inkDark}
      strokeWidth="2"
    />
    <rect
      x="85"
      y="85"
      width="30"
      height="30"
      fill="none"
      stroke={theme.colors.inkLight}
      strokeWidth="1"
    />

    {/* Traces */}
    {[0, 90, 180, 270].map(rot => (
      <g key={rot} transform={`rotate(${rot}, 100, 100)`}>
        {[0, 1, 2, 3].map(i => (
          <motion.line
            key={i}
            x1="100"
            y1="60"
            x2="100"
            y2="20"
            stroke={theme.colors.inkDark}
            strokeWidth="1.5"
            transform={`translate(${(i - 1.5) * 15}, 0)`}
            initial={{ pathLength: 0 }}
            animate={{ pathLength: 1 }}
            transition={{ duration: 0.5, delay: i * 0.1 }}
          />
        ))}
      </g>
    ))}

    {/* Pulse */}
    <motion.rect
      x="60"
      y="60"
      width="80"
      height="80"
      fill="none"
      stroke={theme.colors.accent}
      strokeWidth="2"
      animate={{ opacity: [0, 1, 0] }}
      transition={{ duration: 0.5, repeat: Infinity, repeatDelay: 0.1 }}
    />
  </svg>
);

// ERA 4: ETHER / MESH (INOS)
const EtherEra = () => (
  <svg width="200" height="200" viewBox="0 0 200 200">
    {/* Central Node */}
    <circle cx="100" cy="100" r="8" fill={theme.colors.inkDark} />

    {/* Satellite Nodes */}
    {[0, 60, 120, 180, 240, 300].map((deg, i) => (
      <motion.g key={i}>
        <motion.line
          x1="100"
          y1="100"
          x2="100"
          y2="40"
          stroke={theme.colors.inkLight}
          strokeWidth="1"
          transform={`rotate(${deg}, 100, 100)`}
          initial={{ pathLength: 0 }}
          animate={{ pathLength: 1 }}
          transition={{ duration: 0.8, delay: i * 0.1 }}
        />
        <motion.circle
          cx="100"
          cy="40"
          r="5"
          fill="none"
          stroke={theme.colors.inkDark}
          strokeWidth="2"
          transform={`rotate(${deg}, 100, 100)`}
          initial={{ scale: 0 }}
          animate={{ scale: 1 }}
          transition={{ duration: 0.4, delay: 0.8 + i * 0.1 }}
        />
      </motion.g>
    ))}

    {/* Connecting the outer ring */}
    <motion.circle
      cx="100"
      cy="100"
      r="60"
      fill="none"
      stroke={theme.colors.accent}
      strokeWidth="1"
      strokeDasharray="4 4"
      initial={{ opacity: 0, rotate: 0 }}
      animate={{ opacity: 1, rotate: 360 }}
      transition={{ duration: 10, repeat: Infinity, ease: 'linear' }}
    />
  </svg>
);

export default function MysticLoader({ status }: MysticLoaderProps) {
  const [eraIndex, setEraIndex] = useState(0);

  useEffect(() => {
    // If we are booting/initializing, cycle through history
    // If generic 'loading', keep cycling
    const interval = setInterval(() => {
      setEraIndex(prev => {
        // If we reach the end (Ether), and system is still not ready, loop?
        // Or loop strictly through 0-3
        return (prev + 1) % ERA_SEQUENCE.length;
      });
    }, 2000); // 2 seconds per era

    return () => clearInterval(interval);
  }, []);

  // Force 'Ether' state immediately if status is ready/active (optional polish)
  // For now, let the journey play out or loop

  const currentEra = ERA_SEQUENCE[eraIndex];

  return (
    <div
      style={{
        position: 'fixed',
        top: 0,
        left: 0,
        width: '100vw',
        height: '100vh',
        background: theme.colors.paperCream,
        color: theme.colors.inkDark,
        zIndex: 9999,
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
      }}
    >
      {/* 
        Timeline Progress Bar (Top)
      */}
      <div style={{ position: 'absolute', top: 40, display: 'flex', gap: 10 }}>
        {ERA_SEQUENCE.map((e, i) => (
          <div
            key={e}
            style={{
              width: 40,
              height: 4,
              background: i <= eraIndex ? theme.colors.inkDark : theme.colors.borderMedium,
              transition: 'background 0.3s ease',
            }}
          />
        ))}
      </div>

      <AnimatePresence mode="wait">
        <motion.div
          key={currentEra}
          initial={{ opacity: 0, scale: 0.9, y: 10 }}
          animate={{ opacity: 1, scale: 1, y: 0 }}
          exit={{ opacity: 0, scale: 1.1, y: -10 }}
          transition={{ duration: 0.5 }}
        >
          <CenteredFrame>
            {currentEra === 'origins' && <AbacusEra />}
            {currentEra === 'logic' && <LogicEra />}
            {currentEra === 'silicon' && <SiliconEra />}
            {currentEra === 'ether' && <EtherEra />}
          </CenteredFrame>
        </motion.div>
      </AnimatePresence>

      <div style={{ marginTop: 20, textAlign: 'center', height: 40 }}>
        <motion.p
          key={currentEra}
          initial={{ opacity: 0, y: 5 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: -5 }}
          style={{
            fontFamily: theme.fonts.typewriter,
            fontSize: '0.8rem',
            letterSpacing: '0.2em',
            color: theme.colors.inkLight,
          }}
        >
          {ERA_TITLES[currentEra]}
        </motion.p>
      </div>

      {/* Footer Info */}
      <div style={{ position: 'absolute', bottom: 40, opacity: 0.4 }}>
        <p style={{ fontFamily: theme.fonts.typewriter, fontSize: '0.7rem' }}>
          INOS KERNEL {status.toUpperCase()}
        </p>
      </div>
    </div>
  );
}
