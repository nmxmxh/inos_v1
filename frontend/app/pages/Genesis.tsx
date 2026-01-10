import styled from 'styled-components';
import { motion } from 'framer-motion';
import { NavLink } from 'react-router-dom';
import { Style as ManuscriptStyle } from '../styles/manuscript';
import ChapterNav from '../ui/ChapterNav';
import HistoryTimeline from '../illustrations/HistoryTimeline';
import LanguageTriad from '../illustrations/LanguageTriad';
import ScrollReveal from '../ui/ScrollReveal';

const Style = {
  ...ManuscriptStyle,

  Grid: styled.div`
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 3rem;
    margin-top: 4rem;

    @media (max-width: 768px) {
      grid-template-columns: 1fr;
    }
  `,

  Column: styled.div`
    h3 {
      font-size: 1.5rem;
      font-weight: 800;
      margin-bottom: 1rem;
      color: #1e40af;
    }
    p {
      color: #4b5563;
      line-height: 1.7;
    }
  `,

  Headline: styled.h2`
    font-size: 2.25rem;
    font-weight: 800;
    margin-bottom: 1.5rem;
    color: #111827;
    letter-spacing: -0.02em;
  `,

  ContentCard: styled.div`
    background: rgba(255, 255, 255, 0.88);
    backdrop-filter: blur(12px);
    border: 1px solid ${p => p.theme.colors.borderSubtle};
    border-radius: 8px;
    padding: ${p => p.theme.spacing[8]};
    margin: ${p => p.theme.spacing[10]} 0;

    h3 {
      margin-top: 0;
      margin-bottom: ${p => p.theme.spacing[4]};
      font-size: 1.75rem;
      color: #1e40af;
      font-weight: 800;
    }

    p {
      line-height: 1.8;
      margin-bottom: ${p => p.theme.spacing[4]};
      color: #374151;
    }
  `,

  IllustrationContainer: styled.div`
    width: 100%;
    background: rgba(255, 255, 255, 0.92);
    backdrop-filter: blur(12px);
    border: 1px solid ${p => p.theme.colors.borderSubtle};
    border-radius: 8px;
    margin: ${p => p.theme.spacing[8]} 0;
    overflow: hidden;
  `,

  IllustrationHeader: styled.div`
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: ${p => p.theme.spacing[3]} ${p => p.theme.spacing[4]};
    border-bottom: 1px solid ${p => p.theme.colors.borderSubtle};
    background: rgba(0, 0, 0, 0.02);
  `,

  IllustrationTitle: styled.span`
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 10px;
    font-weight: 600;
    color: ${p => p.theme.colors.inkDark};
    text-transform: uppercase;
    letter-spacing: 0.1em;
  `,

  IllustrationCaption: styled.p`
    font-family: ${p => p.theme.fonts.typewriter};
    font-size: 10px;
    color: ${p => p.theme.colors.inkMedium};
    text-align: center;
    padding: ${p => p.theme.spacing[3]};
    margin: 0;
    border-top: 1px solid ${p => p.theme.colors.borderSubtle};
  `,

  SectionDivider: styled.div`
    height: 1px;
    background: linear-gradient(
      to right,
      transparent,
      ${p => p.theme.colors.borderSubtle},
      transparent
    );
    margin: ${p => p.theme.spacing[16]} 0;
  `,

  DeepDiveGrid: styled.div`
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: ${p => p.theme.spacing[4]};
    margin: ${p => p.theme.spacing[5]} 0;
  `,

  DeepDiveLink: styled(NavLink)<{ $color: string }>`
    display: block;
    background: rgba(255, 255, 255, 0.9);
    border: 1px solid ${p => p.$color}30;
    border-left: 3px solid ${p => p.$color};
    border-radius: 6px;
    padding: ${p => p.theme.spacing[4]};
    text-decoration: none;
    transition:
      transform 0.2s,
      box-shadow 0.2s;

    &:hover {
      transform: translateY(-2px);
      box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
    }
  `,

  DeepDiveTitle: styled.div<{ $color: string }>`
    font-weight: 700;
    color: ${p => p.$color};
    margin-bottom: ${p => p.theme.spacing[1]};
    text-transform: uppercase;
    font-size: 0.75rem;
    letter-spacing: 0.05em;
  `,

  DeepDiveDesc: styled.div`
    font-size: ${p => p.theme.fontSizes.sm};
    color: ${p => p.theme.colors.inkMedium};
  `,

  StatHighlight: styled.span`
    font-weight: 800;
    color: #dc2626;
    background: #fef2f2;
    padding: 0.2rem 0.4rem;
    border-radius: 4px;
  `,
};

export default function Genesis() {
  return (
    <Style.BlogContainer>
      <motion.section
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.8 }}
        style={{ marginBottom: '6rem' }}
      >
        <Style.SectionTitle>Chapter 04</Style.SectionTitle>
        <Style.PageTitle>The 30-Year Correction</Style.PageTitle>
        <Style.LeadParagraph>
          In 1996, the path for the internet seemed clear. Sun Microsystems promised that "The
          Network is the Computer." Bell Labs was testing Plan 9 and Inferno—operating systems where
          every resource was a file, and every machine was a node in a unified, shared memory space.
        </Style.LeadParagraph>
        <p style={{ fontSize: '1.4rem', color: '#374151', maxWidth: '750px', fontWeight: 500 }}>
          But the industry took a different path. We built a library of documents instead of a
          global computer.
        </p>
      </motion.section>

      <Style.SectionDivider />

      <ScrollReveal>
        <section>
          <Style.Headline>The Serialization Wall</Style.Headline>
          <div
            style={{
              fontSize: '1.25rem',
              color: '#374151',
              maxWidth: '800px',
              marginBottom: '3rem',
            }}
          >
            <p>
              By choosing the "Message Passing" web (HTTP/JSON), we unwittingly introduced the{' '}
              <strong>Copy Tax</strong>. Every time data moves between components, it must be
              "serialized"—translated from rich memory structures into flat byte streams.
            </p>
            <p>
              This isn't just a minor overhead. Research from <strong>Google</strong> and
              <strong>Facebook</strong> reveals that up to{' '}
              <Style.StatHighlight>6% of all CPU cycles</Style.StatHighlight> in their massive data
              centers are spent solely on Protobuf and Thrift serialization.
            </p>
            <p>
              In the browser, the cost is even more severe. Without SharedArrayBuffer, the
              "Serialization Wall" can consume over 60% of the total execution time during heavy
              data transfers.
            </p>
          </div>
          <Style.IllustrationContainer>
            <Style.IllustrationHeader>
              <Style.IllustrationTitle>
                History_Ref_04 // The Great Decoupling
              </Style.IllustrationTitle>
            </Style.IllustrationHeader>
            <div style={{ padding: '2rem' }}>
              <HistoryTimeline />
            </div>
            <Style.IllustrationCaption>
              Timeline of the architectural fork: 1996 - 2026. Note the divergence where Shared
              Memory was sacrificed for Message Passing.
            </Style.IllustrationCaption>
          </Style.IllustrationContainer>
        </section>
      </ScrollReveal>

      <Style.SectionDivider />

      <ScrollReveal>
        <section>
          <Style.Headline>The Rule of Choice</Style.Headline>
          <div
            style={{
              fontSize: '1.25rem',
              color: '#374151',
              maxWidth: '800px',
              marginBottom: '3rem',
            }}
          >
            <p>
              INOS is a technical correction. It reunites the primitives of computing by using a
              tri-layer architecture that matches the biology of intelligence. We didn't pick these
              languages because they were popular—we picked them because they were the only ones
              that could fulfill the vision.
            </p>
          </div>

          <Style.IllustrationContainer>
            <Style.IllustrationHeader>
              <Style.IllustrationTitle>
                Engine_Schem_01 // The Reactive Triad
              </Style.IllustrationTitle>
            </Style.IllustrationHeader>
            <div style={{ padding: '2rem' }}>
              <LanguageTriad />
            </div>
            <Style.IllustrationCaption>
              Interaction between Go (Orchestrator), Rust (Executor), and JS (Perceiver). Signal
              pulses represent atomic operations in a shared memory pool.
            </Style.IllustrationCaption>
          </Style.IllustrationContainer>

          <Style.Grid>
            <Style.Column>
              <h3>Rust: The Muscle (Execution)</h3>
              <p>
                In the <strong>2024 Stack Overflow Developer Survey</strong>, Rust was voted the
                <strong>Most Loved Language</strong> for the 9th year in a row (72% admiration). We
                use it because it provides <strong>Zero-Cost Abstractions</strong>.
              </p>
              <p>
                When compiled to WebAssembly with <strong>SIMD vectorization</strong>, Rust achieves{' '}
                <Style.StatHighlight>10-15x speedups</Style.StatHighlight> over pure JavaScript for
                physics simulations. It is the raw power of INOS.
              </p>
            </Style.Column>
            <Style.Column>
              <h3>Go: The Brain (Governance)</h3>
              <p>
                Go is the king of concurrency. Its <strong>Preemptive Scheduler</strong> handles
                millions of goroutines with minimal overhead. In INOS, Go doesn't process data; it
                manages the <strong>Economic Soul</strong> and <strong>Scheduling Policy</strong>.
              </p>
              <p>
                By keeping the "Brain" in Go, we gain industrial-grade reliability. It coordinates
                the Rust executors across the Distributed Shared Memory grid without the complexity
                of manual thread management.
              </p>
            </Style.Column>
          </Style.Grid>

          <Style.ContentCard>
            <h3>WebAssembly: The Universal Lung</h3>
            <p>
              WebAssembly (Wasm) is the portability layer that allows INOS to run anywhere. Recent
              2025 benchmarks demonstrate that **Rust compiled to WebAssembly** can achieve
              performance within <strong>1.5x of native speed</strong>, while bypassing the
              performance spikes of garbage-collected runtimes.
            </p>
            <p>
              By combining Rust's execution efficiency with Go's orchestration and Wasm's
              portability, INOS achieves the goal Sun Microsystems missed: a truly decentralized,
              high-performance Network Computer.
            </p>
          </Style.ContentCard>
        </section>
      </ScrollReveal>

      <Style.SectionDivider />

      <Style.ContentCard>
        <h3>Continue the Journey</h3>
        <p>Explore the specific technical pillars of the INOS architecture:</p>
        <Style.DeepDiveGrid>
          <Style.DeepDiveLink to="/deep-dives/zero-copy" $color="#8b5cf6">
            <Style.DeepDiveTitle $color="#8b5cf6">Zero-Copy I/O</Style.DeepDiveTitle>
            <Style.DeepDiveDesc>Pointers over copies</Style.DeepDiveDesc>
          </Style.DeepDiveLink>
          <Style.DeepDiveLink to="/deep-dives/signaling" $color="#dc2626">
            <Style.DeepDiveTitle $color="#dc2626">Epoch Signaling</Style.DeepDiveTitle>
            <Style.DeepDiveDesc>Atomic mutation loops</Style.DeepDiveDesc>
          </Style.DeepDiveLink>
          <Style.DeepDiveLink to="/deep-dives/mesh" $color="#16a34a">
            <Style.DeepDiveTitle $color="#16a34a">P2P Mesh</Style.DeepDiveTitle>
            <Style.DeepDiveDesc>Gossip + DHT</Style.DeepDiveDesc>
          </Style.DeepDiveLink>
          <Style.DeepDiveLink to="/deep-dives/economy" $color="#f59e0b">
            <Style.DeepDiveTitle $color="#f59e0b">Economic Mesh</Style.DeepDiveTitle>
            <Style.DeepDiveDesc>Yield distribution</Style.DeepDiveDesc>
          </Style.DeepDiveLink>
          <Style.DeepDiveLink to="/deep-dives/threads" $color="#00add8">
            <Style.DeepDiveTitle $color="#00add8">Supervisor Threads</Style.DeepDiveTitle>
            <Style.DeepDiveDesc>Genetic coordination</Style.DeepDiveDesc>
          </Style.DeepDiveLink>
          <Style.DeepDiveLink to="/deep-dives/graphics" $color="#ec4899">
            <Style.DeepDiveTitle $color="#ec4899">Graphics Pipeline</Style.DeepDiveTitle>
            <Style.DeepDiveDesc>Instanced GPU compute</Style.DeepDiveDesc>
          </Style.DeepDiveLink>
          <Style.DeepDiveLink to="/deep-dives/database" $color="#10b981">
            <Style.DeepDiveTitle $color="#10b981">Storage</Style.DeepDiveTitle>
            <Style.DeepDiveDesc>BLAKE3 Addressing</Style.DeepDiveDesc>
          </Style.DeepDiveLink>
        </Style.DeepDiveGrid>
      </Style.ContentCard>

      <ChapterNav
        prev={{ to: '/architecture', title: '03. Architecture' }}
        next={{ to: '/cosmos', title: '05. The Cosmos' }}
      />
    </Style.BlogContainer>
  );
}
