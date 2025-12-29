import { useSystemStore } from '../../src/store/system';

export default function ModuleRegistry() {
  const { units } = useSystemStore();

  const moduleList = Object.values(units);

  if (moduleList.length === 0) {
    return (
      <div style={styles.container}>
        <div style={styles.emptyState}>
          <h2 style={styles.emptyTitle}>No Modules Loaded</h2>
          <p style={styles.emptyText}>Waiting for modules to initialize...</p>
        </div>
      </div>
    );
  }

  return (
    <div style={styles.container}>
      <header style={styles.header}>
        <h1 style={styles.title}>Module Registry</h1>
        <p style={styles.subtitle}>
          {moduleList.length} {moduleList.length === 1 ? 'module' : 'modules'} loaded
        </p>
      </header>

      <div style={styles.grid}>
        {moduleList.map(unit => (
          <div key={unit.id} style={styles.card}>
            {/* Header */}
            <div style={styles.cardHeader}>
              <div style={styles.moduleInfo}>
                <h3 style={styles.moduleName}>{unit.id}</h3>
              </div>
              <div style={styles.statusBadge}>
                <div style={styles.statusDot} />
                <span style={styles.statusText}>{unit.active ? 'Active' : 'Inactive'}</span>
              </div>
            </div>

            {/* Capabilities */}
            {unit.capabilities.length > 0 && (
              <div style={styles.section}>
                <h4 style={styles.sectionTitle}>Capabilities ({unit.capabilities.length})</h4>
                <div style={styles.capabilityList}>
                  {unit.capabilities.map((cap, idx) => (
                    <div key={idx} style={styles.capabilityBadge}>
                      {cap}
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

// ========== Styles ==========

const styles: Record<string, React.CSSProperties> = {
  container: {
    width: '100%',
    minHeight: '100vh',
    padding: '2rem',
    background: 'linear-gradient(135deg, #0a0a0a 0%, #1a1a2e 100%)',
    fontFamily: "'Inter', -apple-system, BlinkMacSystemFont, sans-serif",
  },

  header: {
    marginBottom: '2rem',
    textAlign: 'center',
  },

  title: {
    fontSize: '2.5rem',
    fontWeight: '700',
    background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
    WebkitBackgroundClip: 'text',
    WebkitTextFillColor: 'transparent',
    marginBottom: '0.5rem',
  },

  subtitle: {
    fontSize: '1rem',
    color: 'rgba(255, 255, 255, 0.6)',
    fontWeight: '400',
  },

  grid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))',
    gap: '1.5rem',
    maxWidth: '1400px',
    margin: '0 auto',
  },

  card: {
    background: 'rgba(255, 255, 255, 0.05)',
    backdropFilter: 'blur(10px)',
    border: '1px solid rgba(255, 255, 255, 0.1)',
    borderRadius: '16px',
    padding: '1.5rem',
    transition: 'all 0.3s ease',
    boxShadow: '0 8px 32px rgba(0, 0, 0, 0.3)',
  },

  cardHeader: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'flex-start',
    marginBottom: '1.5rem',
    paddingBottom: '1rem',
    borderBottom: '1px solid rgba(255, 255, 255, 0.1)',
  },

  moduleInfo: {
    display: 'flex',
    flexDirection: 'column',
    gap: '0.25rem',
  },

  moduleName: {
    fontSize: '1.25rem',
    fontWeight: '600',
    color: '#ffffff',
    margin: 0,
    textTransform: 'capitalize',
  },

  statusBadge: {
    display: 'flex',
    alignItems: 'center',
    gap: '0.5rem',
    padding: '0.375rem 0.75rem',
    background: 'rgba(34, 197, 94, 0.1)',
    border: '1px solid rgba(34, 197, 94, 0.3)',
    borderRadius: '20px',
  },

  statusDot: {
    width: '8px',
    height: '8px',
    borderRadius: '50%',
    background: '#22c55e',
    boxShadow: '0 0 8px rgba(34, 197, 94, 0.6)',
  },

  statusText: {
    fontSize: '0.75rem',
    color: '#22c55e',
    fontWeight: '500',
  },

  section: {
    marginBottom: '1.25rem',
  },

  sectionTitle: {
    fontSize: '0.875rem',
    fontWeight: '600',
    color: 'rgba(255, 255, 255, 0.7)',
    textTransform: 'uppercase',
    letterSpacing: '0.05em',
    marginBottom: '0.75rem',
  },

  capabilityList: {
    display: 'flex',
    flexWrap: 'wrap',
    gap: '0.5rem',
  },

  capabilityBadge: {
    padding: '0.375rem 0.75rem',
    background:
      'linear-gradient(135deg, rgba(102, 126, 234, 0.2) 0%, rgba(118, 75, 162, 0.2) 100%)',
    border: '1px solid rgba(102, 126, 234, 0.3)',
    borderRadius: '6px',
    fontSize: '0.75rem',
    color: 'rgba(255, 255, 255, 0.9)',
    fontWeight: '500',
  },

  emptyState: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    justifyContent: 'center',
    minHeight: '60vh',
    textAlign: 'center',
  },

  emptyTitle: {
    fontSize: '1.5rem',
    fontWeight: '600',
    color: 'rgba(255, 255, 255, 0.8)',
    marginBottom: '0.5rem',
  },

  emptyText: {
    fontSize: '1rem',
    color: 'rgba(255, 255, 255, 0.5)',
  },
};
