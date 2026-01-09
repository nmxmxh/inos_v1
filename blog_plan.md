# INOS Technical Codex — Implementation Plan

> *Page-by-page storytelling site integrated into the existing frontend, using native SVG + D3 + Three.js illustrations powered by the existing physics and GPU units.*

---

## Architecture Decision

| Decision | Value |
|:---------|:------|
| **Integration** | Existing `frontend/` (not standalone `/site`) |
| **Illustrations** | SVG + D3.js + Three.js (no static images) |
| **Physics Engine** | Use existing `boids`, `physics`, `math` units (no modifications) |
| **GPU Shaders** | Use existing `nbody.wgsl` and other shaders (no modifications) |
| **Routing** | React Router v6 integrated into `App.tsx` |
| **Moonshot** | Final phase (cosmos simulation as crowning demonstration) |

---

## File Structure

```
frontend/
├── app/
│   ├── App.tsx                    # Router + MotionConfig + ThemeProvider
│   ├── main.tsx
│   ├── styles/
│   │   ├── minimal.css            # Existing design system
│   │   ├── motion.ts              # [NEW] Animation timing, easing, variants
│   │   ├── theme.ts               # [NEW] Colors, typography, spacing
│   │   ├── styled.d.ts            # [NEW] TypeScript theme declaration
│   │   └── manuscript.ts          # [NEW] Da Vinci section components
│   ├── hooks/
│   │   └── useReducedMotion.ts    # [NEW] Accessibility hook
│   ├── components/
│   │   ├── ArchitecturalBoids.tsx # Existing boids background
│   │   ├── Layout.tsx             # [NEW] Header, nav, footer + HUD
│   │   ├── PageTransition.tsx     # [NEW] AnimatePresence wrapper
│   │   ├── ScrollReveal.tsx       # [NEW] useInView-based reveals
│   │   ├── Navigation.tsx         # [NEW] Nav with layoutId indicator
│   │   ├── ChapterNav.tsx         # [NEW] Prev/Next with keyboard support
│   │   └── PerformanceHUD.tsx     # [NEW] SAB epoch-driven metrics
│   ├── illustrations/             # [NEW] D3/SVG/Three.js graphics
│   │   ├── CopyTaxDiagram.tsx     # Villain visualization
│   │   ├── LayerArchitecture.tsx  # 3-layer exploded view
│   │   ├── SABMemoryMap.tsx       # Memory layout
│   │   ├── EpochSignaling.tsx     # Before/during/after timeline
│   │   ├── MeshTopology.tsx       # P2P network graph (instanced)
│   │   └── CreditFlow.tsx         # Economic circulation
│   └── pages/                     # [NEW] Chapter pages
│       ├── Landing.tsx            # Home + ToC
│       ├── Problem.tsx            # Ch 1: The Villain
│       ├── Insight.tsx            # Ch 2: The Vision
│       ├── Architecture.tsx       # Ch 3: The System
│       ├── DeepDives/             # Ch 4: Technical pillars (7 pages)
│       │   ├── index.tsx          # Export all deep dives
│       │   ├── ZeroCopy.tsx       # [DONE] SharedArrayBuffer + zero-copy patterns
│       │   ├── Signaling.tsx      # Epoch-based reactive mutation
│       │   ├── Mesh.tsx           # P2P gossip + DHT + reputation
│       │   ├── Economy.tsx        # Credits, storage tiers, yield
│       │   ├── Threads.tsx        # Supervisor architecture (from docs/threads.md)
│       │   ├── Graphics.tsx       # WebGPU pipeline (from docs/graphics.md)
│       │   └── Database.tsx       # SQLite WASM + P2P sync (from docs/database.md)
│       ├── Implementation.tsx     # Ch 5: How it's built
│       ├── Roadmap.tsx            # Ch 6: Where we're going
│       └── Cosmos.tsx             # Ch 7: The Moonshot
└── src/
    └── ... (existing WASM bridge, stores, hooks)
```

---

## Phase Implementation

### Phase 0: Foundation (Design System + Infrastructure)

**Goal:** Establish animation timing system, theme, accessibility, routing, and core layout.

#### 0.1 Dependencies

```bash
cd frontend
npm install react-router-dom framer-motion d3 styled-components
npm install -D @types/d3 @types/styled-components
```

#### 0.2 Animation Timing System

| Category | Duration | Use Case |
|:---------|:---------|:---------|
| `MICRO` | 180ms | Button clicks, toggles, icon changes |
| `STANDARD` | 250ms | Card reveals, dropdowns, modals |
| `PAGE` | 350ms | Page transitions |
| `EMPHASIS` | 500ms | Hero animations, attention-grabbing |
| `STAGGER` | 50ms | Delay between staggered list items |

**Easing Curves (Material Design 3 inspired):**
- `standard`: `[0.4, 0.0, 0.2, 1.0]` — Most UI transitions
- `emphasized`: `[0.0, 0.0, 0.2, 1.0]` — Dramatic, hero elements
- `decelerate`: `[0.0, 0.0, 0.0, 1.0]` — Elements entering
- `accelerate`: `[0.4, 0.0, 1.0, 1.0]` — Elements exiting
- `spring`: `{ type: 'spring', stiffness: 300, damping: 30 }` — Interactive

**Manuscript-Specific Variants:**
- `manuscript`: Ink reveal with blur transition
- `blueprint`: Technical clip-path reveal

#### 0.3 Theme Architecture (Da Vinci Manuscript)

| Token | Manuscript | Blueprint Variant |
|:------|:-----------|:------------------|
| **Paper** | `#f4f1ea` (cream) | `#dbeafe` (light blue) |
| **Ink** | `#1a1a1a` (dark) | `#1e40af` (blueprint) |
| **Accent** | `#6d28d9` (purple) | `#1e40af` (blue) |
| **Grid** | None | 20px repeating pattern |

#### 0.4 Accessibility Requirements (WCAG AA)

| Requirement | Implementation |
|:------------|:---------------|
| `prefers-reduced-motion` | `useReducedMotion` hook + MotionConfig |
| Keyboard navigation | Focus indicators, arrow key support |
| No autoplay >5s | Pauseable animations |
| No >3 flashes/sec | Validated in all animations |
| Focus visible | `:focus-visible` outline styles |

#### 0.5 Phase 0 Tasks

| Task | File | Description |
|:-----|:-----|:------------|
| [ ] Install deps | `package.json` | Add all dependencies |
| [ ] Create motion system | `styles/motion.ts` | TIMING, EASING, TRANSITIONS, VARIANTS |
| [ ] Create theme | `styles/theme.ts` | Colors, fonts, spacing, shadows |
| [ ] Create styled.d.ts | `styles/styled.d.ts` | TypeScript theme declaration |
| [ ] Create manuscript styles | `styles/manuscript.ts` | ManuscriptSection, BlueprintSection, AnnotationSticker |
| [ ] Create useReducedMotion | `hooks/useReducedMotion.ts` | Accessibility hook with localStorage override |
| [ ] Create Layout | `components/Layout.tsx` | ThemeProvider + MotionConfig + header/footer |
| [ ] Create PageTransition | `components/PageTransition.tsx` | AnimatePresence + location key |
| [ ] Create ScrollReveal | `components/ScrollReveal.tsx` | useInView trigger |
| [ ] Create Navigation | `components/Navigation.tsx` | layoutId indicator animation |
| [ ] Create ChapterNav | `components/ChapterNav.tsx` | Prev/Next + keyboard arrows |
| [ ] Update App.tsx | `app/App.tsx` | BrowserRouter + routes |
| [ ] Add blog styles | `styles/minimal.css` | .performance-hud, .illustration-container |

**Verification:**
- [ ] `npm run dev` starts without errors
- [ ] Routes render placeholder content
- [ ] Navigation works between pages
- [ ] Page transitions animate (and skip with reduced motion)
- [ ] Boids background continues to work on landing
- [ ] Keyboard navigation (Tab, Enter, Arrow keys) works

---

### Phase 1: Landing Page

**Goal:** Create the entry point with table of contents.

| Task | File | Description |
|:-----|:-----|:------------|
| [ ] Create Landing | `pages/Landing.tsx` | Hero + animated title |
| [ ] Add ToC | `pages/Landing.tsx` | Chapter links with d3 icons |
| [ ] Mini boids | `illustrations/MiniBoids.tsx` | Small Three.js embed in hero |
| [ ] Performance HUD | `components/PerformanceHUD.tsx` | Live metrics from SAB |

**Illustrations:**
- Animated title with subtle glow
- Mini boids canvas (100 birds, loop animation)
- ToC icons as SVG with d3 transitions

**Content from MANIFESTO:**
> *"The future of computing isn't built in data centers. It's woven into the fabric of everyday devices."*

---

### Phase 2: Chapter 1 — The Problem

**Goal:** Introduce the villain (The Copy Tax).

| Task | File | Description |
|:-----|:-----|:------------|
| [ ] Create Problem page | `pages/Problem.tsx` | Carnegie hook + villain narrative |
| [ ] Copy Tax diagram | `illustrations/CopyTaxDiagram.tsx` | D3 cluttered pipeline |
| [ ] Latency sparkline | `illustrations/SparklineLatency.tsx` | D3 inline sparkline |
| [ ] Statistics grid | `pages/Problem.tsx` | MANIFESTO's energy/performance stats |

**Illustrations:**

1. **CopyTaxDiagram.tsx** — D3 SVG
   - Nodes: Microservices, threads, processes
   - Edges: Animated data blobs with "COPY" labels
   - Color: Red/orange for serialization points
   - Animation: Data flows left-to-right, slowing at each copy

2. **SparklineLatency.tsx** — D3 SVG
   - Inline sparkline: latency grows with node count
   - No axes, just the trend line
   - Tufte-style minimal

**Content from MANIFESTO (sections I-IV):**
- Energy Crisis: 200+ TWh data centers, 150 TWh Bitcoin
- Performance Ceiling: Moore's Law dying, 300ms latency
- Innovation Drought: Stagnation since 2005
- Bitcoin's Broken Promise: 99.9% waste

**Carnegie Hook:**
> *"If you've ever watched your application stall because messages are still being decoded, you know the tax we're paying."*

---

### Phase 3: Chapter 2 — The Insight

**Goal:** Reveal the vision (The Hero).

| Task | File | Description |
|:-----|:-----|:------------|
| [ ] Create Insight page | `pages/Insight.tsx` | Guiding questions + technology convergence |
| [ ] Blood vs Bottled | `illustrations/BloodFlowDiagram.tsx` | Traditional vs INOS metaphor |
| [ ] Technology stack | `pages/Insight.tsx` | WebAssembly, WebGPU, WebRTC, SAB |

**Illustrations:**

1. **BloodFlowDiagram.tsx** — D3 SVG side-by-side
   - Left: Fragmented organs with bottles being passed (traditional)
   - Right: Unified bloodstream flowing through all organs (INOS)
   - Animation: Smooth flow on right, stuttering on left

**Content from MANIFESTO (sections V-VI):**
- Web 1.0 → 2.0 → 3.0 table
- Technology convergence: WebAssembly, WebGPU, WebRTC, SAB, Service Workers
- Personal curiosity narrative

**Jobs Tagline:**
> *"We're not building websites anymore. We're building living systems."*

---

### Phase 4: Chapter 3 — The Architecture

**Goal:** Show the 3-layer structure with biological metaphors.

| Task | File | Description |
|:-----|:-----|:------------|
| [ ] Create Architecture page | `pages/Architecture.tsx` | Rule of Three + exploded diagram |
| [ ] Layer diagram | `illustrations/LayerArchitecture.tsx` | D3 exploded view |
| [ ] Boids integration | `pages/Architecture.tsx` | Reuse ArchitecturalBoids (proof) |

**Illustrations:**

1. **LayerArchitecture.tsx** — D3 SVG
   - Layer 1 (Host/Body): Nginx + JS Bridge
   - Layer 2 (Kernel/Brain): Go WASM
   - Layer 3 (Modules/Muscle): Rust WASM
   - Direct annotations on each layer
   - Biological labels: Body, Brain, Muscle

**Live Demo:**
- Embed `ArchitecturalBoids` as proof of zero-copy rendering
- Show bird count, ops/second from SAB epoch counters

**Content from inos_context.json:**
```
Rule of Three:
- Layer 1: The Body (Nginx + JS Bridge) — Speed & Sensors
- Layer 2: The Brain (Go Kernel) — Orchestration & Policy
- Layer 3: The Muscle (Rust Modules) — Compute & Storage
```

---

### Phase 5: Chapter 4 — Deep Dives (4 pages)

**Goal:** Technical pillars with interactive visualizations.

#### 4.1 Zero-Copy I/O

| Task | File | Description |
|:-----|:-----|:------------|
| [ ] Create ZeroCopy page | `pages/DeepDives/ZeroCopy.tsx` | SAB explanation |
| [ ] Memory map | `illustrations/SABMemoryMap.tsx` | D3 memory layout |
| [ ] Pointer animation | `illustrations/PointerVsCopy.tsx` | D3 animated comparison |

**Illustration: SABMemoryMap.tsx**
- Visual representation of sab_layout.capnp offsets
- Color-coded regions (flags, buffers, matrices)
- Hover to reveal size and purpose

**Headline:** *"Data in Motion, Without the Copy."*

---

#### 4.2 Epoch Signaling

| Task | File | Description |
|:-----|:-----|:------------|
| [ ] Create Signaling page | `pages/DeepDives/Signaling.tsx` | Reactive patterns |
| [ ] Epoch timeline | `illustrations/EpochSignaling.tsx` | D3 small multiples |

**Illustration: EpochSignaling.tsx**
- Three frames: Before → During → After
- Epoch counter incrementing
- Observers reacting in parallel

**Headline:** *"React to Reality, Not to Messages."*

---

#### 4.3 P2P Mesh

| Task | File | Description |
|:-----|:-----|:------------|
| [ ] Create Mesh page | `pages/DeepDives/Mesh.tsx` | Gossip + reputation |
| [ ] Network graph | `illustrations/MeshTopology.tsx` | D3 force-directed |
| [ ] Gossip animation | `illustrations/GossipSpread.tsx` | D3 propagation waves |

**Illustration: MeshTopology.tsx**
- Force-directed graph with nodes sized by reputation
- Edges pulse when gossip propagates
- Live simulation using d3-force

**Headline:** *"Trust, Verified by the Network."*

---

#### 4.4 Economic Storage

| Task | File | Description |
|:-----|:-----|:------------|
| [ ] Create Economy page | `pages/DeepDives/Economy.tsx` | Credits + hot/cold tiers |
| [ ] Credit flow | `illustrations/CreditFlow.tsx` | D3 sankey diagram |

**Illustration: CreditFlow.tsx**
- Sankey diagram: Work → Credits → Storage
- Hot/Cold tier visualization
- Color gradient from hot (red) to cold (blue)

**Headline:** *"Storage That Pays for Itself."*

---

#### 4.5 Supervisor Threads (NEW)

| Task | File | Description |
|:-----|:-----|:------------|
| [ ] Create Threads page | `pages/DeepDives/Threads.tsx` | Supervisor architecture |
| [ ] Supervisor hierarchy | `illustrations/SupervisorTree.tsx` | D3 tree diagram |
| [ ] Genetic algorithm flow | `illustrations/GeneticFlow.tsx` | D3 animated evolution |

**Source Documentation:** `docs/threads.md`

**Content:**
- Supervisor as "Intelligent Managers" (not just process monitors)
- Five responsibilities: Manage, Learn, Optimize, Schedule, Secure
- Hierarchical architecture (RootSupervisor → Domain → Unit)
- Genetic algorithm coordination for population management
- Pattern storage and cross-supervisor knowledge sharing

**Illustration: SupervisorTree.tsx**
- Collapsible tree with RootSupervisor at root
- Domain supervisors as branches
- Unit supervisors as leaves
- Color by health status (green/yellow/red)

**Headline:** *"Intelligence at Every Level."*

---

#### 4.6 Graphics Pipeline (NEW)

| Task | File | Description |
|:-----|:-----|:------------|
| [ ] Create Graphics page | `pages/DeepDives/Graphics.tsx` | WebGPU + instanced rendering |
| [ ] Pipeline diagram | `illustrations/GpuPipeline.tsx` | D3 flow chart |
| [ ] Instance batching | `illustrations/InstancedMesh.tsx` | D3 animated batching |

**Source Documentation:** `docs/graphics.md`

**Content:**
- WebGPU fundamentals (GPU programming model, bind groups, pipelines)
- Instanced rendering pattern (10k+ entities at 60fps)
- Transform matrix flow (CPU → SAB → GPU)
- Depth sorting with atomics
- Culling strategies (frustum, occlusion, distance)
- Ping-pong buffer pattern for render/compute separation

**Illustration: GpuPipeline.tsx**
- Flow: Positions (SAB) → Vertex Shader → Rasterizer → Fragment Shader → Screen
- Animated data packets flowing through pipeline
- Highlight critical path

**Headline:** *"GPU-Native from Day One."*

---

#### 4.7 Distributed Database (NEW)

| Task | File | Description |
|:-----|:-----|:------------|
| [ ] Create Database page | `pages/DeepDives/Database.tsx` | SQLite WASM + P2P sync |
| [ ] Sync diagram | `illustrations/DatabaseSync.tsx` | D3 peer synchronization |
| [ ] CRDT merge | `illustrations/CrdtMerge.tsx` | D3 animated conflict resolution |

**Source Documentation:** `docs/database.md`

**Content:**
- SQLite WASM as the local database engine
- Content-addressed storage (BLAKE3 hashing)
- CRDT-based conflict resolution
- Gossip protocol for metadata propagation
- Hot/Cold tier data migration
- Schema evolution and migrations

**Illustration: DatabaseSync.tsx**
- Multiple browser nodes with SQLite icons
- Arrows showing bidirectional sync
- Highlight conflict resolution moments

**Headline:** *"Your Database, Everywhere."*

---

### Phase 6: Chapter 5 — Implementation

**Goal:** How it's built, for engineers.

| Task | File | Description |
|:-----|:-----|:------------|
| [ ] Create Implementation page | `pages/Implementation.tsx` | Build system, tools |
| [ ] Build graph | `illustrations/BuildGraph.tsx` | D3 DAG of compilation |

**Content:**
- Cap'n Proto schemas → Rust/Go/TypeScript
- WASM compilation pipeline
- SAB memory layout specification

---

### Phase 7: Chapter 6 — Roadmap

**Goal:** Where we're going.

| Task | File | Description |
|:-----|:-----|:------------|
| [ ] Create Roadmap page | `pages/Roadmap.tsx` | Timeline with milestones |
| [ ] Timeline | `illustrations/RoadmapTimeline.tsx` | D3 horizontal timeline |

**Milestones:**
- ✅ Zero-copy reactive signaling
- ✅ Content-addressed storage mesh
- ✅ Economic incentive layer
- 🚀 Proof-of-Useful-Work consensus
- 🚀 Federated learning across mesh
- 🚀 Global compute marketplace

---

### Phase 8: Chapter 7 — The Moonshot (Cosmos)

**Goal:** Crown demonstration — real-time cosmological simulation.

| Task | File | Description |
|:-----|:-----|:------------|
| [ ] Create Cosmos page | `pages/Cosmos.tsx` | Galaxy simulation |
| [ ] Galaxy renderer | `illustrations/CosmosRenderer.tsx` | Three.js + existing nbody.wgsl |
| [ ] Fidelity scaling | `pages/Cosmos.tsx` | Particles scale with mock nodes |
| [ ] Live dashboard | `pages/Cosmos.tsx` | Node count, particles, interactions |

**Technical Integration:**
- Reuse existing `modules/compute/src/units/gpu_shaders/nbody.wgsl`
- Use `physics.nbody_step_enhanced` capability
- Scale particle count based on simulated node availability
- Three.js InstancedMesh for rendering (same pattern as boids)

**The "One More Thing" Quote:**
> *"Today, simulating the birth of a galaxy takes the world's fastest supercomputers months.*
>
> *What if a million browsers could do it in real-time?*
>
> *That's the INOS moonshot."*

---

## Routing Configuration

```tsx
// App.tsx
<BrowserRouter>
  <Routes>
    <Route path="/" element={<Layout />}>
      <Route index element={<Landing />} />
      <Route path="problem" element={<Problem />} />
      <Route path="insight" element={<Insight />} />
      <Route path="architecture" element={<Architecture />} />
      <Route path="deep-dives">
        <Route path="zero-copy" element={<ZeroCopy />} />
        <Route path="signaling" element={<Signaling />} />
        <Route path="mesh" element={<Mesh />} />
        <Route path="economy" element={<Economy />} />
        <Route path="threads" element={<Threads />} />
        <Route path="graphics" element={<Graphics />} />
        <Route path="database" element={<Database />} />
      </Route>
      <Route path="implementation" element={<Implementation />} />
      <Route path="roadmap" element={<Roadmap />} />
      <Route path="cosmos" element={<Cosmos />} />
    </Route>
  </Routes>
</BrowserRouter>
```

---

## Illustration Patterns

### D3 SVG Pattern

```tsx
// illustrations/ExampleDiagram.tsx
import { useRef, useEffect } from 'react';
import * as d3 from 'd3';

export function ExampleDiagram({ data }) {
  const svgRef = useRef<SVGSVGElement>(null);
  
  useEffect(() => {
    if (!svgRef.current) return;
    
    const svg = d3.select(svgRef.current);
    svg.selectAll('*').remove(); // Clean slate
    
    // D3 rendering logic...
    // - Maximize data-ink ratio (Tufte)
    // - Direct annotation (Da Vinci)
    // - Semantic colors only
    
    return () => svg.selectAll('*').remove();
  }, [data]);
  
  return (
    <div className="illustration-container">
      <svg ref={svgRef} viewBox="0 0 800 400" />
    </div>
  );
}
```

### Three.js Pattern (for GPU-powered illustrations)

```tsx
// illustrations/GpuDrivenIllustration.tsx
import { Canvas, useFrame } from '@react-three/fiber';
import { useSystemStore } from '../../src/store/system';
import { dispatch } from '../../src/wasm/dispatch';

function GpuRenderer() {
  const { moduleExports } = useSystemStore();
  
  useFrame((_, delta) => {
    if (moduleExports?.compute) {
      // Use existing units without modification
      dispatch.execute('physics', 'nbody_step', { dt: delta });
    }
  });
  
  return <instancedMesh ... />;
}

export function GpuDrivenIllustration() {
  return (
    <Canvas camera={{ position: [0, 0, 50] }}>
      <GpuRenderer />
    </Canvas>
  );
}
```

---

## Performance HUD Specification

Visible on every page, reading directly from SAB:

```tsx
// components/PerformanceHUD.tsx
import { useSystemStore } from '../../src/store/system';
import { useEffect, useState } from 'react';

export function PerformanceHUD() {
  const [metrics, setMetrics] = useState({ ops: 0, epoch: 0 });
  
  useEffect(() => {
    const sab = (window as any).__INOS_SAB__;
    if (!sab) return;
    
    const interval = setInterval(() => {
      const flags = new Int32Array(sab, 0, 16);
      setMetrics({
        epoch: Atomics.load(flags, 0),
        ops: Atomics.load(flags, 1),
      });
    }, 100);
    
    return () => clearInterval(interval);
  }, []);
  
  return (
    <div className="performance-hud">
      <span>EPOCH: {metrics.epoch}</span>
      <span>OPS: {metrics.ops}</span>
    </div>
  );
}
```

---

## Execution Order

```
┌─────────────────────────────────────────────────────────┐
│  Phase 0: Foundation                                    │
│  ─────────────────────────────────────────────────────  │
│  [ ] Install dependencies                               │
│  [ ] Create Layout, PageTransition, ChapterNav          │
│  [ ] Update App.tsx with routing                        │
│  [ ] Verify navigation works                            │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│  Phase 1: Landing                                       │
│  ─────────────────────────────────────────────────────  │
│  [ ] Landing page with hero                             │
│  [ ] Table of contents with icons                       │
│  [ ] PerformanceHUD component                           │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│  Phase 2: Problem (The Villain)                         │
│  ─────────────────────────────────────────────────────  │
│  [ ] Problem page content                               │
│  [ ] CopyTaxDiagram (D3 SVG)                            │
│  [ ] SparklineLatency (D3 SVG)                          │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│  Phase 3: Insight (The Vision)                          │
│  ─────────────────────────────────────────────────────  │
│  [ ] Insight page content                               │
│  [ ] BloodFlowDiagram (D3 SVG)                          │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│  Phase 4: Architecture (The System)                     │
│  ─────────────────────────────────────────────────────  │
│  [ ] Architecture page content                          │
│  [ ] LayerArchitecture diagram (D3 SVG)                 │
│  [ ] Embed ArchitecturalBoids as proof                  │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│  Phase 5: Deep Dives (4 pillars)                        │
│  ─────────────────────────────────────────────────────  │
│  [ ] Zero-Copy + SABMemoryMap                           │
│  [ ] Signaling + EpochSignaling                         │
│  [ ] Mesh + MeshTopology                                │
│  [ ] Economy + CreditFlow                               │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│  Phase 6: Implementation                                │
│  ─────────────────────────────────────────────────────  │
│  [ ] Implementation page + BuildGraph                   │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│  Phase 7: Roadmap                                       │
│  ─────────────────────────────────────────────────────  │
│  [ ] Roadmap page + Timeline                            │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│  Phase 8: Cosmos (The Moonshot)                         │
│  ─────────────────────────────────────────────────────  │
│  [ ] Cosmos page                                        │
│  [ ] CosmosRenderer using existing nbody.wgsl           │
│  [ ] Fidelity scaling dashboard                         │
│  [ ] "One More Thing" narrative                         │
└─────────────────────────────────────────────────────────┘
```

---

## Dependencies

```bash
cd frontend
npm install react-router-dom framer-motion d3 styled-components
npm install -D @types/d3 @types/styled-components
```

---

## INOS Architecture Integrations

Novel opportunities from `docs/graphics.md` to leverage the SAB-native architecture:

| Pattern | Current Use | Blog Application |
|:--------|:------------|:-----------------|
| **Epoch Counters** | Physics sync | Page transition orchestration via `IDX_PAGE_EPOCH` |
| **Ping-Pong Buffers** | Render/compute | Double-buffered page content preloading |
| **Instanced Rendering** | 10k boids | Instanced D3 particles for MeshTopology (1000+ nodes @ 60fps) |
| **Zero-CPU Idle** | `Atomics.wait` | Visibility-aware animations (battery-conscious) |
| **Context Versioning** | Zombie killing | Navigation guards, stale animation cleanup |
| **Persistent Scratch** | WASM modules | Cached D3 selections, reused SVG elements |

### Integration Examples

**1. Epoch-Driven Performance HUD (Zero Polling)**
```typescript
// Read real metrics directly from SAB epochs
const flags = new Int32Array(sab, 0, 16);
const epoch = Atomics.load(flags, IDX_BIRD_EPOCH);
const matrixEpoch = Atomics.load(flags, IDX_MATRIX_EPOCH);
```

**2. Instanced D3 Particles (graphics.md Pattern)**
```typescript
// Apply Three.js InstancedMesh pattern to D3
// Pre-allocate circles, update transforms from SAB
const nodes = svg.selectAll('circle')
  .data(new Float32Array(sab, MESH_OFFSET, nodeCount * 3));
```

**3. Context Versioning for Animation Cleanup**
```typescript
// Zombie-kill stale animations on navigation
const contextId = window.__INOS_CONTEXT_ID__;
useEffect(() => {
  return () => {
    if (window.__INOS_CONTEXT_ID__ !== contextId) return;
    // Cleanup animations
  };
}, []);
```

---

## Quality Checklist (Per Phase)

### Da Vinci (Visual)
- [ ] Does the illustration reveal the mechanism's essence?
- [ ] Is every element necessary for understanding?
- [ ] Are text and image integrated?

### Carnegie (Connection)
- [ ] Does the page acknowledge the audience's challenge?
- [ ] Is every feature described as a benefit?
- [ ] Are objections addressed proactively?

### Jobs (Narrative)
- [ ] Is there a clear villain → hero arc?
- [ ] Is the Rule of Three applied?
- [ ] Is the core message in one sentence?

### Tufte (Precision)
- [ ] Is the data-ink ratio maximized?
- [ ] Are annotations direct on graphics?
- [ ] Is there zero chartjunk?

### Accessibility (WCAG)
- [ ] Does the page respect `prefers-reduced-motion`?
- [ ] Are all interactive elements keyboard accessible?
- [ ] Do focus indicators work without relying on animation?
- [ ] Are ARIA labels present where needed?

---

*Start with Phase 0 when ready. The canvas is prepared—now we paint.*

