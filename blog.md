# INOS Technical Codex — Blog Site Implementation

> A conversational, illustrative technical whitepaper site built with Vite, React Router, D3, and styled-components.

---

## Vision

The site reads like **Da Vinci's Codex**—each page is a chapter in a beautifully illustrated technical narrative. Not marketing, not documentation. A **technical blog** that tells the story of INOS through prose, diagrams, and interactive demos.

**Design Language**: Based on `frontend/app/styles/minimal.css`
- Paper-cream background (#f4f1ea)
- Inter typeface (display + typewriter)
- Purple accent (#6d28d9)
- Jotter-style sections with left border
- Noise texture overlay
- Blog container (800px max-width)

---

## Tech Stack

| Layer | Choice | Rationale |
|:------|:-------|:----------|
| Build | **Vite** | Fast, modern, already used in frontend |
| Routing | **React Router v6** | Nested routes, transitions |
| Styling | **styled-components** | CSS-in-JS, theming, no Tailwind |
| Graphics | **D3.js** | Data-driven SVG illustrations |
| Animation | **Framer Motion** | Page transitions, micro-interactions |
| Content | **MDX** (optional) | Markdown + React for chapters |
| Base CSS | `minimal.css` | Extend existing jotter aesthetic |

---

## Site Structure

```
site/
├── src/
│   ├── main.tsx
│   ├── App.tsx                    # Router + transitions
│   ├── styles/
│   │   ├── theme.ts               # Styled-components theme
│   │   └── GlobalStyle.ts         # Based on minimal.css
│   ├── components/
│   │   ├── Layout.tsx             # Header, nav, footer
│   │   ├── PageTransition.tsx     # Framer Motion wrapper
│   │   ├── PerformanceHUD.tsx     # Live metrics bar
│   │   ├── Illustration.tsx       # D3 wrapper component
│   │   └── CodeBlock.tsx          # Syntax highlighting
│   ├── illustrations/             # D3 graphic components
│   │   ├── LayerDiagram.tsx       # 3-layer architecture
│   │   ├── CopyTax.tsx            # Villain visualization
│   │   ├── SABMemoryMap.tsx       # Memory layout
│   │   ├── EpochSignaling.tsx     # Before/during/after
│   │   ├── MeshTopology.tsx       # P2P network graph
│   │   ├── CreditFlow.tsx         # Economic circulation
│   │   └── BoidsEmbed.tsx         # Reuse from frontend
│   └── pages/
│       ├── index.tsx              # Landing + TOC
│       ├── problem.tsx            # Ch 1
│       ├── insight.tsx            # Ch 2
│       ├── architecture.tsx       # Ch 3 + Boids
│       ├── system.tsx             # Modules, Units, Threads live dashboard
│       ├── deep-dives/
│       │   ├── index.tsx          # Deep-dive hub
│       │   ├── zero-copy.tsx
│       │   ├── signaling.tsx
│       │   ├── mesh.tsx
│       │   └── economy.tsx
│       ├── implementation.tsx     # Ch 5
│       ├── roadmap.tsx            # Ch 6
│       └── demo.tsx               # Full interactive
└── public/
    └── fonts/                     # Inter fonts
```

---

## Route Configuration

```tsx
<Routes>
  <Route path="/" element={<Layout />}>
    <Route index element={<Landing />} />
    <Route path="problem" element={<Problem />} />
    <Route path="insight" element={<Insight />} />
    <Route path="architecture" element={<Architecture />} />
    <Route path="system" element={<SystemDashboard />} />
    <Route path="deep-dives">
      <Route index element={<DeepDivesHub />} />
      <Route path="zero-copy" element={<ZeroCopy />} />
      <Route path="signaling" element={<Signaling />} />
      <Route path="mesh" element={<Mesh />} />
      <Route path="economy" element={<Economy />} />
    </Route>
    <Route path="implementation" element={<Implementation />} />
    <Route path="roadmap" element={<Roadmap />} />
    <Route path="demo" element={<Demo />} />
  </Route>
</Routes>
```

---

## Page Illustrations

Each page has a **hero illustration** and **sub-illustrations**:

| Page | Hero Illustration | Sub-Illustrations |
|:-----|:------------------|:------------------|
| **Landing** | Animated logo/boids mini | TOC icons for each chapter |
| **Problem** | "Copy Tax" data flow (cluttered) | Sparkline: latency growth |
| **Insight** | Traditional vs INOS comparison | Bloodstream metaphor |
| **Architecture** | Exploded 3-layer diagram | Interactive boids embed |
| **Zero-Copy** | SAB memory map | Pointer vs copy animation |
| **Signaling** | Epoch counter timeline | Small multiples (before/during/after) |
| **Mesh** | P2P network graph | Gossip propagation animation |
| **Economy** | Credit circulation flow | Hot/cold tier visual |
| **Implementation** | Code flow diagram | Build system graph |
| **Roadmap** | Timeline with milestones | Status indicators |
| **Demo** | Full boids canvas | Live metrics overlay |

---

## Performance HUD (The Selling Point)

A persistent metrics bar visible on every page—showing what the distributed runtime can do **right now**.

### Header HUD (Compact)

```
┌────────────────────────────────────────────────────────────────────────┐
│  INOS                                                     [Demo] [Docs]│
│  ─────────────────────────────────────────────────────────────────────│
│  🟢 47,832 Nodes  │  12.4 PFLOPS  │  847M ops/s  │  ⚡ 23ms latency    │
└────────────────────────────────────────────────────────────────────────┘
```

### Footer HUD (Expanded)

```
┌────────────────────────────────────────────────────────────────────────┐
│  SYSTEM STATUS                                                         │
│  ───────────────────────────────────────────────────────────────────── │
│  NODES           COMPUTE           THROUGHPUT        LATENCY           │
│  47,832          12.4 PFLOPS       847M ops/s        23ms              │
│  ↑ 156/hour      ████████░░        ████████████░     P99: 89ms         │
│                                                                        │
│  MODULES ACTIVE                    CREDITS FLOWING                     │
│  gpu: 12,847  │  compute: 8,234   1.4M credits/min                    │
│  storage: 4,123  │  ml: 2,891                                          │
│                                                                        │
│  [+ Add Your Node]                                                     │
└────────────────────────────────────────────────────────────────────────┘
```

### Metrics Tracked

| Metric | Source |
|:-------|:-------|
| **Active Nodes** | P2P mesh heartbeat |
| **Compute Power** | GPU capability reports (FLOPS) |
| **Ops/Second** | Epoch counter deltas |
| **Avg/P99 Latency** | Job completion timestamps |
| **Module Activity** | Supervisor heartbeats |
| **Credits Flow** | Ledger transactions |

---

## /system Page — Architecture Dashboard

A dedicated live page showing modules, units, and thread architecture.

### Sections

1. **Layer Overview** — Interactive D3 3-layer diagram
2. **Active Modules Table** — Status, units, jobs/min, latency per module
3. **Unit Registry** — From `inos_context.json`, filterable by module
4. **Thread Architecture** — Supervisor hierarchy, SAB memory layout
5. **Live Metrics Graphs** — D3 sparklines for throughput, latency histograms

---

## D3 Integration Pattern

Following `graphics.md` principles (zero-alloc, SAB-aware):

```tsx
// illustrations/LayerDiagram.tsx
import * as d3 from 'd3';
import { useRef, useEffect } from 'react';
import styled from 'styled-components';

const SVGContainer = styled.div`
  width: 100%;
  max-width: 600px;
  margin: 2rem auto;
`;

export const LayerDiagram = ({ data }) => {
  const svgRef = useRef<SVGSVGElement>(null);
  
  useEffect(() => {
    if (!svgRef.current) return;
    
    const svg = d3.select(svgRef.current);
    // D3 rendering logic...
    // Direct annotation (Tufte principle)
    // Minimal chrome (maximize data-ink ratio)
    
    return () => svg.selectAll('*').remove();
  }, [data]);
  
  return (
    <SVGContainer>
      <svg ref={svgRef} viewBox="0 0 600 400" />
    </SVGContainer>
  );
};
```

---

## Page Transitions

```tsx
// components/PageTransition.tsx
import { motion } from 'framer-motion';

const pageVariants = {
  initial: { opacity: 0, y: 20 },
  animate: { opacity: 1, y: 0 },
  exit: { opacity: 0, y: -20 }
};

export const PageTransition = ({ children }) => (
  <motion.div
    variants={pageVariants}
    initial="initial"
    animate="animate"
    exit="exit"
    transition={{ duration: 0.3, ease: 'easeInOut' }}
  >
    {children}
  </motion.div>
);
```

---

## Styled-Components Theme

Based on `minimal.css` variables:

```tsx
// styles/theme.ts
export const theme = {
  colors: {
    paperCream: '#f4f1ea',
    paperWhite: '#ffffff',
    inkDark: '#1a1a1a',
    inkMedium: '#404040',
    inkLight: '#737373',
    accent: '#6d28d9',
    accentDim: '#ede9fe',
    border: '#e5e5e5',
  },
  fonts: {
    main: "'Inter', -apple-system, system-ui, sans-serif",
    typewriter: "'Inter', ui-monospace, monospace",
  },
  spacing: {
    section: '6rem',
    container: '800px',
  },
};
```

---

## Chapter Content Structure

Each page follows the Renaissance Communicator pattern:

```tsx
// pages/problem.tsx
export const Problem = () => (
  <PageTransition>
    <BlogSection>
      {/* Carnegie: Empathy hook */}
      <p className="lead">
        If you've ever built a distributed system, you know the feeling...
      </p>
      
      {/* Da Vinci: Hero illustration */}
      <Illustration>
        <CopyTaxDiagram />
      </Illustration>
      
      {/* Jobs: Name the villain */}
      <JotterSection>
        <JotterNumber>THE VILLAIN</JotterNumber>
        <JotterHeading>The Copy Tax</JotterHeading>
        <p>Every copy costs you. Every message adds latency...</p>
      </JotterSection>
      
      {/* Tufte: Data proof */}
      <SparklineInline trend="up" label="Latency as nodes scale" />
      
      {/* Navigation */}
      <ChapterNav prev={null} next="/insight" />
    </BlogSection>
  </PageTransition>
);
```

---

## Build & Development

```bash
# Create site
npm create vite@latest site -- --template react-ts
cd site

# Dependencies
npm install react-router-dom styled-components framer-motion d3
npm install -D @types/d3 @types/styled-components

# Development
npm run dev

# Build
npm run build
```

---

## Performance Considerations

Following `graphics.md` principles:

1. **Lazy load D3 illustrations** — Split code per page
2. **Preload fonts** — Inter Display via font-display: swap
3. **Cache boids component** — Reuse Three.js context on demo page
4. **Minimize transitions** — 300ms max, ease-in-out
5. **Static generation** — Consider Vite SSG plugin for production

---

## Navigation Design

A minimal left sidebar or top navigation:

```
┌────────────────────────────────────────────┐
│  INOS                    [Demo]            │
├────────────────────────────────────────────┤
│                                            │
│  ← Previous    Chapter 1: The Problem  →   │
│                                            │
│  [Content...]                              │
│                                            │
│  ──────────────────────────────────────    │
│                                            │
│  TABLE OF CONTENTS                         │
│  • The Problem                             │
│  • The Insight                             │
│  • The Architecture                        │
│  • Deep Dives →                            │
│  • Implementation                          │
│  • Roadmap                                 │
│                                            │
└────────────────────────────────────────────┘
```

---

## 🐋 The Moonshot: Real-Time Cosmological Simulation

The site's aspirational goal—the **Moby Dick** that proves the architecture's power.

### The Problem

Simulate **10¹² - 10¹⁵ gravitational particles** in real-time:
- Galaxy formation from primordial dark matter halos
- Galaxy collisions, mergers, cosmic web structure
- Currently takes supercomputers **months**
- INOS goal: **real-time with distributed browser nodes**

### Why This Is Perfect

| Criteria | Fit |
|:---------|:----|
| **Visualizable** | Galaxies forming—stunningly beautiful |
| **Compute-only** | Force = G·m₁·m₂/r², state = position/velocity/mass |
| **Minimal storage** | 1T particles × 28 bytes ≈ 28TB distributed RAM |
| **Already built** | `gpu_shaders/nbody.wgsl` already has galaxy arms, dark matter, black holes |
| **Scales with nodes** | Barnes-Hut O(N log N), trivially parallelizable |

### Existing Infrastructure

**Already implemented in `modules/compute/src/units/gpu_shaders/nbody.wgsl`:**

```wgsl
struct Particle {
    position: vec3<f32>,
    velocity: vec3<f32>,
    mass: f32,
    particle_type: u32,  // 0=normal, 1=star, 2=blackhole, 3=dark_matter
    temperature: f32,
    luminosity: f32,
    // ... 16 fields total
}
```

**Features already working:**
- ✅ Multiple force laws (Newtonian, Plummer, Logarithmic)
- ✅ Galaxy arm density functions
- ✅ Dark matter factor
- ✅ Black hole accretion disks
- ✅ Collision detection & merging
- ✅ Tidal forces
- ✅ Temperature → color (blackbody radiation)
- ✅ Cosmic expansion parameter
- ✅ Workgroup-optimized tile-based force calculation

### Continuous Scaling Model

Particle count grows dynamically with available nodes:

```
Particles = Σ(Node GPU Budget) / Cost Per Particle
```

| Nodes | Particles | Visual Fidelity |
|:------|:----------|:----------------|
| 1 (local) | ~10,000 | Dust cloud |
| 100 | ~1,000,000 | Spiral arms visible |
| 10,000 | ~100,000,000 | Galaxy cluster |
| 1,000,000 | ~10,000,000,000+ | Cosmic web |

### Proposed Unit: `cosmos`

A dedicated unit for distributed cosmological simulation:

```
modules/compute/src/units/cosmos.rs
```

**Capabilities (planned):**
- `cosmos.init_universe` — Spawn particles with initial conditions
- `cosmos.step` — Compute one timestep across distributed nodes
- `cosmos.partition` — Barnes-Hut spatial partitioning
- `cosmos.merge_state` — Aggregate particle updates from peers
- `cosmos.get_snapshot` — Return current particle state for rendering

**Integration with existing units:**

| Unit | Role in Cosmos |
|:-----|:---------------|
| `gpu` | Execute `nbody.wgsl` compute shaders |
| `boids` | Flocking algorithm (already N-body lite) |
| `storage` | Persist simulation checkpoints |
| `crypto` | Sign particle state for P2P verification |

### The Live Dashboard

```
┌─────────────────────────────────────────────┐
│  INOS COSMOS                                │
│                                             │
│  [Live Galaxy Visualization]                │
│                                             │
│  ─────────────────────────────────────────  │
│                                             │
│  Active Nodes     2,847    ↑ 23 this hour   │
│  Particles        284.7M   (+100K/node)     │
│  Interactions/s   4.2T                      │
│  Fidelity         ████████░░  78%           │
│                                             │
│  [+ Add Your Node]                          │
│                                             │
└─────────────────────────────────────────────┘
```

### The "One More Thing"

> "Today, simulating the birth of a galaxy takes the world's fastest supercomputers months.
> 
> What if a million browsers could do it in real-time?
> 
> **That's the INOS moonshot.**"

---

## Next Steps

1. [ ] Initialize Vite project in `/site`
2. [ ] Port `minimal.css` to styled-components theme
3. [ ] Create Layout + PageTransition components
4. [ ] Build Landing page with TOC
5. [ ] Create first D3 illustration (LayerDiagram)
6. [ ] Build Problem chapter as template
7. [ ] Port boids to standalone embed
8. [ ] Complete remaining chapters
