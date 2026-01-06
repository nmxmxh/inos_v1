package sab

import (
	"encoding/binary"
	"fmt"
)

// SABInitializer handles SAB initialization on kernel startup
type SABInitializer struct {
	buffer    []byte
	size      int
	validator *SABValidator
}

// NewSABInitializer creates a new SAB initializer
func NewSABInitializer(size int) (*SABInitializer, error) {
	if size < int(SAB_SIZE_MIN) {
		return nil, fmt.Errorf("SAB size %d is below minimum %d", size, SAB_SIZE_MIN)
	}

	if size > int(SAB_SIZE_MAX) {
		return nil, fmt.Errorf("SAB size %d exceeds maximum %d", size, SAB_SIZE_MAX)
	}

	// Create buffer
	buffer := make([]byte, size)

	// Create validator
	validator := NewSABValidator(uint32(size))

	return &SABInitializer{
		buffer:    buffer,
		size:      size,
		validator: validator,
	}, nil
}

// Initialize performs complete SAB initialization
func (si *SABInitializer) Initialize() error {
	// 1. Register all memory regions
	if err := si.registerRegions(); err != nil {
		return fmt.Errorf("failed to register regions: %w", err)
	}

	// 2. Initialize metadata region
	if err := si.initMetadata(); err != nil {
		return fmt.Errorf("failed to initialize metadata: %w", err)
	}

	// 3. Initialize module registry
	if err := si.initModuleRegistry(); err != nil {
		return fmt.Errorf("failed to initialize module registry: %w", err)
	}

	// 4. Initialize supervisor headers
	if err := si.initSupervisorHeaders(); err != nil {
		return fmt.Errorf("failed to initialize supervisor headers: %w", err)
	}

	// 5. Initialize pattern exchange
	if err := si.initPatternExchange(); err != nil {
		return fmt.Errorf("failed to initialize pattern exchange: %w", err)
	}

	// 6. Initialize job history
	if err := si.initJobHistory(); err != nil {
		return fmt.Errorf("failed to initialize job history: %w", err)
	}

	// 7. Initialize coordination state
	if err := si.initCoordination(); err != nil {
		return fmt.Errorf("failed to initialize coordination: %w", err)
	}

	// 8. Initialize inbox/outbox
	if err := si.initInboxOutbox(); err != nil {
		return fmt.Errorf("failed to initialize inbox/outbox: %w", err)
	}

	// 9. Initialize arena
	if err := si.initArena(); err != nil {
		return fmt.Errorf("failed to initialize arena: %w", err)
	}

	// 10. Validate layout
	if err := si.validator.ValidateLayout(); err != nil {
		return fmt.Errorf("layout validation failed: %w", err)
	}

	// 11. Set kernel ready flag
	si.setKernelReady()

	return nil
}

// registerRegions registers all memory regions with the validator
func (si *SABInitializer) registerRegions() error {
	regions := []struct {
		name    string
		offset  uint32
		size    uint32
		purpose string
	}{
		{"AtomicFlags", OFFSET_ATOMIC_FLAGS, SIZE_ATOMIC_FLAGS, "Epoch counters and atomic flags"},
		{"SupervisorAlloc", OFFSET_SUPERVISOR_ALLOC, SIZE_SUPERVISOR_ALLOC, "Dynamic epoch allocation table"},
		{"ModuleRegistry", OFFSET_MODULE_REGISTRY, SIZE_MODULE_REGISTRY, "Module metadata and capabilities"},
		{"SupervisorHeaders", OFFSET_SUPERVISOR_HEADERS, SIZE_SUPERVISOR_HEADERS, "Supervisor state headers"},
		{"PatternExchange", OFFSET_PATTERN_EXCHANGE, SIZE_PATTERN_EXCHANGE, "Learned patterns and optimizations"},
		{"JobHistory", OFFSET_JOB_HISTORY, SIZE_JOB_HISTORY, "Job execution history"},
		{"Coordination", OFFSET_COORDINATION, SIZE_COORDINATION, "Cross-unit coordination state"},
		{"InboxOutbox", OFFSET_INBOX_OUTBOX, SIZE_INBOX_OUTBOX, "Job request/result communication"},
		{"Arena", OFFSET_ARENA, CalculateArenaSize(uint32(si.size)), "Dynamic allocation for overflow"},
	}

	for _, r := range regions {
		if err := si.validator.RegisterRegion(r.name, r.offset, r.size, r.purpose); err != nil {
			return err
		}
	}

	return nil
}

// initMetadata initializes the metadata region (atomic flags + allocation table)
func (si *SABInitializer) initMetadata() error {
	// Zero out atomic flags region
	for i := OFFSET_ATOMIC_FLAGS; i < OFFSET_ATOMIC_FLAGS+SIZE_ATOMIC_FLAGS; i++ {
		si.buffer[i] = 0
	}

	// Initialize system epoch counters to 0
	// IDX_KERNEL_READY = 0 (not ready yet)
	// IDX_SENSOR_EPOCH, IDX_ACTOR_EPOCH, etc. = 0

	// Zero out supervisor allocation table
	for i := OFFSET_SUPERVISOR_ALLOC; i < OFFSET_SUPERVISOR_ALLOC+SIZE_SUPERVISOR_ALLOC; i++ {
		si.buffer[i] = 0
	}

	// Initialize allocation table structure
	// Bitmap (16 bytes) - all zeros (all indices free)
	// NextIndex (4 bytes) - set to SUPERVISOR_POOL_BASE
	offset := OFFSET_SUPERVISOR_ALLOC + 16
	binary.LittleEndian.PutUint32(si.buffer[offset:offset+4], SUPERVISOR_POOL_BASE)

	// AllocatedCount (4 bytes) - set to 0
	offset += 4
	binary.LittleEndian.PutUint32(si.buffer[offset:offset+4], 0)

	// Allocations count (4 bytes) - set to 0
	offset += 4
	binary.LittleEndian.PutUint32(si.buffer[offset:offset+4], 0)

	return nil
}

// initModuleRegistry initializes the module registry region
func (si *SABInitializer) initModuleRegistry() error {
	// Zero out entire registry region
	for i := OFFSET_MODULE_REGISTRY; i < OFFSET_MODULE_REGISTRY+SIZE_MODULE_REGISTRY; i++ {
		si.buffer[i] = 0
	}

	// Modules will self-register by writing their entries
	// No pre-initialization needed beyond zeroing

	return nil
}

// initSupervisorHeaders initializes the supervisor headers region
func (si *SABInitializer) initSupervisorHeaders() error {
	// Zero out entire supervisor headers region
	for i := OFFSET_SUPERVISOR_HEADERS; i < OFFSET_SUPERVISOR_HEADERS+SIZE_SUPERVISOR_HEADERS; i++ {
		si.buffer[i] = 0
	}

	// Supervisors will be created dynamically
	// No pre-initialization needed beyond zeroing

	return nil
}

// initPatternExchange initializes the pattern exchange region
func (si *SABInitializer) initPatternExchange() error {
	// Zero out entire pattern exchange region
	for i := OFFSET_PATTERN_EXCHANGE; i < OFFSET_PATTERN_EXCHANGE+SIZE_PATTERN_EXCHANGE; i++ {
		si.buffer[i] = 0
	}

	return nil
}

// initJobHistory initializes the job history region
func (si *SABInitializer) initJobHistory() error {
	// Zero out entire job history region
	for i := OFFSET_JOB_HISTORY; i < OFFSET_JOB_HISTORY+SIZE_JOB_HISTORY; i++ {
		si.buffer[i] = 0
	}

	// Initialize circular buffer metadata (head, tail, count)
	// First 12 bytes: [head:4] [tail:4] [count:4]
	offset := OFFSET_JOB_HISTORY
	binary.LittleEndian.PutUint32(si.buffer[offset:offset+4], 0)    // head
	binary.LittleEndian.PutUint32(si.buffer[offset+4:offset+8], 0)  // tail
	binary.LittleEndian.PutUint32(si.buffer[offset+8:offset+12], 0) // count

	return nil
}

// initCoordination initializes the coordination state region
func (si *SABInitializer) initCoordination() error {
	// Zero out entire coordination region
	for i := OFFSET_COORDINATION; i < OFFSET_COORDINATION+SIZE_COORDINATION; i++ {
		si.buffer[i] = 0
	}

	return nil
}

// initInboxOutbox initializes the inbox/outbox communication regions
func (si *SABInitializer) initInboxOutbox() error {
	// Zero out entire inbox/outbox region
	for i := OFFSET_INBOX_OUTBOX; i < OFFSET_INBOX_OUTBOX+SIZE_INBOX_OUTBOX; i++ {
		si.buffer[i] = 0
	}

	return nil
}

// initArena initializes the arena region
func (si *SABInitializer) initArena() error {
	// Zero out entire arena region
	arenaSize := CalculateArenaSize(uint32(si.size))
	for i := uint32(0); i < arenaSize; i++ {
		si.buffer[OFFSET_ARENA+i] = 0
	}

	// Arena will be managed by buddy allocator
	// No pre-initialization needed beyond zeroing

	return nil
}

// setKernelReady sets the kernel ready flag
func (si *SABInitializer) setKernelReady() {
	// Set IDX_KERNEL_READY (index 0) to 1
	offset := OFFSET_ATOMIC_FLAGS
	binary.LittleEndian.PutUint32(si.buffer[offset:offset+4], 1)
}

// GetBuffer returns the initialized SAB buffer
func (si *SABInitializer) GetBuffer() []byte {
	return si.buffer
}

// GetValidator returns the validator
func (si *SABInitializer) GetValidator() *SABValidator {
	return si.validator
}

// GetMemoryMap returns a human-readable memory map
func (si *SABInitializer) GetMemoryMap() string {
	return si.validator.GetMemoryMap()
}

// InitializationStats holds initialization statistics
type InitializationStats struct {
	TotalSize      int
	RegionsCount   int
	ArenaSize      uint32
	MemoryMap      string
	ValidationPass bool
}

// GetStats returns initialization statistics
func (si *SABInitializer) GetStats() InitializationStats {
	return InitializationStats{
		TotalSize:      si.size,
		RegionsCount:   len(si.validator.regions),
		ArenaSize:      CalculateArenaSize(uint32(si.size)),
		MemoryMap:      si.GetMemoryMap(),
		ValidationPass: si.validator.ValidateLayout() == nil,
	}
}
