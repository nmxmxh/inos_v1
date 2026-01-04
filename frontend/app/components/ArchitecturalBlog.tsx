export default function ArchitecturalBlog() {
  return (
    <div className="blog-container">
      <header className="blog-header">
        <div className="jotter-handwriting">ARCHITECTURAL MANIFESTO</div>
        <h1 className="jotter-title" style={{ fontSize: '4rem', marginBottom: '1rem' }}>
          The Internet-Native Operating System
        </h1>
        <p className="jotter-handwriting">V2.0 | JANUARY 2026</p>
      </header>

      <section className="blog-section">
        <h2 className="jotter-heading">I. The Silence of the Machines</h2>
        <p>
          We inhabit a world of quiet excess. In our pockets, on our desks, and carried in our bags
          lie billions of idle supercomputers. They wait for a signal that rarely comes, their
          silicon potential wasted on scrolling pixels while massive data centers burn fossil fuels
          to handle the world's compute.
        </p>
        <p>
          INOS was born from a simple, radical question:{' '}
          <i>What if compute was a shared resource, like oxygen?</i>
        </p>
      </section>

      <section className="blog-section">
        <h2 className="jotter-heading">II. The Zero-Copy Paradigm</h2>
        <p>
          Traditional systems are built on conversation—serializing data, sending it over a network,
          deserializing it, and reacting. This is slow. It's the speed of light bounded by the
          friction of translation.
        </p>
        <p>
          INOS replaces conversation with <b>Shared Reality</b>. Through{' '}
          <code>SharedArrayBuffer</code>, multiple languages—Go, Rust, and JavaScript—look at the
          same memory at the same time. There is no serialization. There is only mutation and
          signaling.
        </p>
        <div className="blog-metric-grid">
          <div className="blog-metric">
            <span className="blog-metric-label">Latency</span>
            <span className="blog-metric-value">0ms</span>
            <span className="jotter-handwriting">Serialization Overhead</span>
          </div>
          <div className="blog-metric">
            <span className="blog-metric-label">Efficiency</span>
            <span className="blog-metric-value">100%</span>
            <span className="jotter-handwriting">Zero-Copy Handoff</span>
          </div>
        </div>
      </section>

      <section className="blog-section">
        <h2 className="jotter-heading">III. The Tri-Layer Architecture</h2>
        <p>INOS is structured like a biological organism, not a mechanical construct.</p>
        <ul className="jotter-list">
          <li>
            <b>Layer 1: The Body (Host)</b> — Nginx and the Browser Bridge. Handling the raw ingress
            of the world.
          </li>
          <li>
            <b>Layer 2: The Brain (Kernel)</b> — Go-based orchestration. Managing the economy, the
            mesh, and the policy.
          </li>
          <li>
            <b>Layer 3: The Muscle (Modules)</b> — Rust-based compute. Heavy lifting, cryptography,
            and the physics of the system.
          </li>
        </ul>
      </section>

      <section className="blog-section">
        <h2 className="jotter-heading">IV. Birds Learning to Fly</h2>
        <p>
          Look at the background. Those aren't just pixels. They are a swarm of autonomous agents
          undergoing evolution. Every flap of their wings is a physics calculation inside a Rust
          module, signaled to the browser through atomic epochs.
        </p>
        <p>
          They learn to flock, they learn to survive. They represent the emergent intelligence of a
          distributed runtime that doesn't just execute code—it <b>lives</b>.
        </p>
      </section>

      <footer className="blog-footer">
        <p className="jotter-handwriting">Built with 🧠 and ❤️ by The INOS Architects</p>
        <button
          className="minimal-button primary"
          onClick={() => window.scrollTo({ top: 0, behavior: 'smooth' })}
        >
          Return to Hub
        </button>
      </footer>
    </div>
  );
}
