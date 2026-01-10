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
│       ├── Architecture.tsx       # Ch 3: The System (incl. Implementation)
│       ├── Genesis.tsx            # [NEW] Ch 4: The 30-Year Correction
│       ├── DeepDives/             # Technical pillars (7 pages)
│       │   ├── index.tsx          # Export all deep dives
│       │   ├── ZeroCopy.tsx       # [DONE] SharedArrayBuffer + zero-copy patterns
│       │   ├── Signaling.tsx      # Epoch-based reactive mutation
│       │   ├── Mesh.tsx           # P2P gossip + DHT + reputation
│       │   ├── Economy.tsx        # Credits, storage tiers, yield
│       │   ├── Threads.tsx        # Supervisor architecture (from docs/threads.md)
│       │   ├── Graphics.tsx       # WebGPU pipeline (from docs/graphics.md)
│       │   └── Database.tsx       # SQLite WASM + P2P sync (from docs/database.md)
│       └── Cosmos.tsx             # Ch 5: The Moonshot (incl. Roadmap)
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
| [x] Install deps | `package.json` | Add all dependencies |
| [x] Create motion system | `styles/motion.ts` | TIMING, EASING, TRANSITIONS, VARIANTS |
| [x] Create theme | `styles/theme.ts` | Colors, fonts, spacing, shadows |
| [x] Create styled.d.ts | `styles/styled.d.ts` | TypeScript theme declaration |
| [x] Create manuscript styles | `styles/manuscript.ts` | ManuscriptSection, BlueprintSection, AnnotationSticker |
| [x] Create useReducedMotion | `hooks/useReducedMotion.ts` | Accessibility hook with localStorage override |
| [x] Create Layout | `components/Layout.tsx` | ThemeProvider + MotionConfig + header/footer |
| [x] Create PageTransition | `components/PageTransition.tsx` | AnimatePresence + location key |
| [x] Create ScrollReveal | `ui/ScrollReveal.tsx` | useInView trigger |
| [x] Create Navigation | `components/Navigation.tsx` | layoutId indicator animation |
| [x] Create ChapterNav | `ui/ChapterNav.tsx` | Prev/Next + keyboard arrows |
| [x] Update App.tsx | `app/App.tsx` | BrowserRouter + routes |
| [x] Add blog styles | `styles/minimal.css` | .performance-hud, .illustration-container |

**Verification:**
- [x] `npm run dev` starts without errors
- [x] Routes render placeholder content
- [x] Navigation works between pages
- [x] Page transitions animate (and skip with reduced motion)
- [x] Boids background continues to work on landing
- [x] Keyboard navigation (Tab, Enter, Arrow keys) works

---

### Phase 1: Landing Page

**Goal:** Create the entry point with table of contents.

| Task | File | Description |
|:-----|:-----|:------------|
| [x] Create Landing | `pages/Landing.tsx` | Hero + animated title |
| [x] Add ToC | `pages/Landing.tsx` | Chapter links with d3 icons |
| [x] Mini boids | `Landing.tsx` | Integrated into background |
| [x] Performance HUD | `Landing.tsx` | LiveStatsGrid as live metrics from SAB |

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
| [x] Create Problem page | `pages/Problem.tsx` | Carnegie hook + villain narrative |
| [x] Centralization Tax | `illustrations/CentralizationDiagram.tsx` | D3 hub-spoke model |
| [x] Copy Tax diagram | `illustrations/CopyTaxDiagram.tsx` | D3 interactive scenarios |
| [x] Statistics grid | `pages/Problem.tsx` | Energy/performance metrics |

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
| [x] Create Insight page | `pages/Insight.tsx` | Guiding questions + technology convergence |
| [x] Wasm Comparison | `illustrations/WasmComparisonDiagram.tsx` | Traditional vs INOS layer view |
| [x] Boids Data Flow | `illustrations/BoidsDataFlowDiagram.tsx` | Pipeline: Compute -> Learn -> Render |
| [x] Technology stack | `pages/Insight.tsx` | WebAssembly, WebGPU, WebRTC, SAB |

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

### Phase 4: Chapter 3 — The Architecture & Implementation

**Goal:** Show the 3-layer structure and technical build process.

| Task | File | Description |
|:-----|:-----|:------------|
| [x] Update Architecture page | `pages/Architecture.tsx` | Rule of Three + Implementation Details |
| [x] Three-Layer Diagram | `illustrations/ThreeLayerDiagram.tsx` | D3 exploded conceptual view |
| [x] SAB Memory Map | `illustrations/SABMemoryMapDiagram.tsx` | Memory layout and offsets |
| [x] Library Proxy | `illustrations/LibraryProxyDiagram.tsx` | Go/Rust binding pattern |
| [x] Build Pipeline | `illustrations/BuildPipelineDiagram.tsx` | Schema-first compilation flow |
| [x] Boids integration | `pages/Architecture.tsx` | Reuse ArchitecturalBoids (proof) |

**Illustrations:**

1. **LayerArchitecture.tsx** — D3 SVG
   - Layer 1 (Host/Body): Nginx + JS Bridge
   - Layer 2 (Kernel/Brain): Go WASM
   - Layer 3 (Modules/Muscle): Rust WASM
   - Direct annotations on each layer
   - Biological labels: Body, Brain, Muscle

2. **BuildGraph.tsx** — D3 SVG
   - DAG: Cap'n Proto Schemas -> Code Generation -> WASM Compilation -> Browser Linking
   - Showing the zero-copy lineage

**Live Demo:**
- Embed `ArchitecturalBoids` as proof of zero-copy rendering
- Show bird count, ops/second from SAB epoch counters

**Content:**
- Rule of Three: Body, Brain, Muscle.
- Implementation Pipeline: How Rust, Go, and TS are bound by a single schema.

---

### Phase 5: Chapter 4 — Genesis (NEW)

**Goal:** Document the architectural "Wrong Turn" and the technical correction.

| Task | File | Description |
|:-----|:-----|:------------|
| [x] Create Genesis page | `pages/Genesis.tsx` | The history of the Decoupling |
| [x] History Timeline | `illustrations/HistoryTimeline.tsx` | 1996 -> 2026 technical fork |
| [x] Language Triad | `illustrations/LanguageTriad.tsx` | Go, Rust, and JS rationale |

**Illustrations:**

1. **HistoryTimeline.tsx** — D3 SVG
   - The fork: Shared Memory (The Path Not Taken) vs Message Passing (The Web)
   - 1996: Plan 9, Java vision, Inferno
   - 2026: WASM, SAB, INOS reconciliation

2. **LanguageTriad.tsx** — D3 SVG
   - Triangle: Brain (Go), Muscle (Rust), Body (JS)
   - Connection points: Signaling, Credits, Identity
   - Hover to see why each language was chosen (Go: Scheduling, Rust: Memory Safety, JS: Ingress)

**Content:**
- The 30-year correction: Why we abandoned the global computer ideal.
- The Serialization Tax: How JSON broke the speed of thought.
- Rule of Choice: Why Go for the Brain, Rust for the Muscle, JS for the Body.

---

### Phase 6: Chapter 5 — Deep Dives (8 pages)

**Goal:** Technical pillars with interactive visualizations.

#### 4.1 Zero-Copy I/O

| Task | File | Description |
|:-----|:-----|:------------|
| [x] Create ZeroCopy page | `pages/DeepDives/ZeroCopy.tsx` | SAB explanation |
| [x] Copy Tax Diagram | `illustrations/CopyTaxDiagram.tsx` | D3 comparison between trad vs zero-copy |
| [x] Memory map | `illustrations/SABMemoryMapDiagram.tsx` | D3 memory layout (in Architecture) |

**Illustration: SABMemoryMap.tsx**
- Visual representation of sab_layout.capnp offsets
- Color-coded regions (flags, buffers, matrices)
- Hover to reveal size and purpose

**Headline:** *"Data in Motion, Without the Copy."*

---

#### 4.2 Epoch Signaling

| Task | File | Description |
|:-----|:-----|:------------|
| [x] Create Signaling page | `pages/DeepDives/Signaling.tsx` | Reactive patterns |
| [x] Epoch comparison | `illustrations/ParadigmCard.tsx` | D3 Polling vs Atomics vs Epochs |

**Illustration: EpochSignaling.tsx**
- Three frames: Before → During → After
- Epoch counter incrementing
- Observers reacting in parallel

**Headline:** *"React to Reality, Not to Messages."*

---

#### 4.3 P2P Mesh

| Task | File | Description |
|:-----|:-----|:------------|
| [x] Create Mesh page | `pages/DeepDives/Mesh.tsx` | Gossip + reputation |
| [x] Network graph | `illustrations/HierarchicalMeshDiagram.tsx` | D3 hierarchical mesh |
| [x] Gossip animation | `illustrations/GossipDiagram.tsx` | D3 propagation waves |

**Illustration: MeshTopology.tsx**
- Force-directed graph with nodes sized by reputation
- Edges pulse when gossip propagates
- Live simulation using d3-force

**Headline:** *"Trust, Verified by the Network."*

---

#### 4.4 Economic Storage

| Task | File | Description |
|:-----|:-----|:------------|
| [x] Create Economy page | `pages/DeepDives/Economy.tsx` | Credits + participation economy |
| [x] Contribution flow | `illustrations/EconomyLogic.tsx` | UBI + yield distribution |

**Illustration: CreditFlow.tsx**
- Sankey diagram: Work → Credits → Storage
- Hot/Cold tier visualization
- Color gradient from hot (red) to cold (blue)

**Headline:** *"Storage That Pays for Itself."*

---

#### 4.5 Supervisor Threads (NEW)

| Task | File | Description |
|:-----|:-----|:------------|
| [x] Create Threads page | `pages/DeepDives/Threads.tsx` | Supervisor architecture |
| [x] Supervisor hierarchy | `illustrations/SupervisorHierarchyDiagram.tsx` | D3 tree diagram |
| [x] Execution flow | `illustrations/JobExecutionFlowDiagram.tsx` | D3 animated job lifecycle |

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
| [x] Create Graphics page | `pages/DeepDives/Graphics.tsx` | WebGPU + instanced rendering |
| [x] Engine integration | `illustrations/TerrainScene.tsx` | Three.js + boids overlay |
| [x] Transform flow | `illustrations/PipelineDiagram.tsx` | D3 data flow through pipeline |

**Source Documentation:** `docs/graphics.md`

**Content:**
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
| [x] Create Database page | `pages/DeepDives/Database.tsx` | Persistence paradoxes |
| [x] Tiered storage | `illustrations/HotColdStorage.tsx` | D3 data migration visualization |
| [x] Block hashing | `illustrations/BlockHashingMap.tsx` | 1MB chunking visual |

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

### Phase 7: Chapter 5 — The Moonshot & Roadmap (Cosmos)

**Goal:** Crown demonstration (real-time cosmos simulation) + vision for the future.

| Task | File | Description |
|:-----|:-----|:------------|
| [x] Update Cosmos page | `pages/Cosmos.tsx` | Galaxy simulation + Roadmap milestones |
| [x] Scale Map | `illustrations/SupercomputerDensityMap.tsx` | Scale comparison visualization |
| [x] Bridge Diagram | `illustrations/RoboticProtocolBridge.tsx` | Cross-reality protocol bridge |
| [x] Roadmap Timeline | `illustrations/RoadmapTimeline.tsx` | D3 horizontal milestone timeline |
| [ ] Galaxy renderer | `illustrations/CosmosRenderer.tsx` | Three.js + existing nbody.wgsl |
| [ ] Fidelity scaling | `pages/Cosmos.tsx` | Particles scale with mock nodes |

**Milestones for Roadmap:**
- ✅ Zero-copy reactive signaling
- ✅ Content-addressed storage mesh
- ✅ Economic incentive layer
- 🚀 Proof-of-Useful-Work consensus
- 🚀 Federated learning across mesh
- 🚀 Global compute marketplace

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
      <Route path="genesis" element={<Genesis />} />
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
│  [x] Install dependencies                               │
│  [x] Create Layout, PageTransition, ChapterNav          │
│  [x] Update App.tsx with routing                        │
│  [x] Verify navigation works                            │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│  Phase 1: Landing                                       │
│  ─────────────────────────────────────────────────────  │
│  [x] Landing page with hero                             │
│  [x] Table of contents with icons                       │
│  [x] PerformanceHUD component                           │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│  Phase 2: Problem (The Villain)                         │
│  ─────────────────────────────────────────────────────  │
│  [x] Problem page content                               │
│  [x] CentralizationDiagram                              │
│  [x] CopyTaxDiagram                                     │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│  Phase 3: Insight (The Vision)                          │
│  ─────────────────────────────────────────────────────  │
│  [x] Insight page content                               │
│  [x] WasmComparisonDiagram                              │
│  [x] BoidsDataFlowDiagram                               │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│  Phase 4: Architecture & Implementation                 │
│  ─────────────────────────────────────────────────────  │
│  [x] Architecture page content                          │
│  [x] ThreeLayerDiagram / SABMemoryMap                   │
│  [x] BuildPipelineDiagram                               │
│  [x] Embed ArchitecturalBoids as proof                  │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│  Phase 5: Genesis (The 30-Year Correction)               │
│  ─────────────────────────────────────────────────────  │
│  [x] Genesis page content                               │
│  [x] HistoryTimeline                                    │
│  [x] LanguageTriad                                      │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│  Phase 6: Deep Dives (7 technical pillars)              │
│  ─────────────────────────────────────────────────────  │
│  [x] Zero-Copy + SABMemoryMap                           │
│  [x] Signaling + EpochSignaling                         │
│  [/] Mesh + MeshTopology                                │
│  [/] Economy + CreditFlow                               │
│  [/] Threads + SupervisorTree                           │
│  [/] Graphics + GpuPipeline                             │
│  [/] Database + DatabaseSync                            │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│  Phase 7: Cosmos (The Moonshot & Roadmap)               │
│  ─────────────────────────────────────────────────────  │
│  [x] Cosmos page + RoadmapTimeline                      │
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

