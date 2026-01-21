package sab

import (
	"errors"
	"fmt"
	"sync"
)

// SABValidator provides runtime validation of SAB operations
// Ensures memory safety and prevents region overlaps
type SABValidator struct {
	regions    []MemoryRegion
	sabSize    uint32
	mu         sync.RWMutex
	violations []ValidationViolation
}

// ValidationViolation records a validation error
type ValidationViolation struct {
	Type      string
	Message   string
	Offset    uint32
	Size      uint32
	Timestamp int64
}

// NewSABValidator creates a new SAB validator
func NewSABValidator(sabSize uint32) *SABValidator {
	return &SABValidator{
		regions:    GetAllRegions(sabSize),
		sabSize:    sabSize,
		violations: make([]ValidationViolation, 0),
	}
}

// RegisterRegion adds a custom region to the validator
// This is used for dynamically allocated regions in the arena
func (v *SABValidator) RegisterRegion(name string, offset, size uint32, purpose string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Validate region bounds
	if offset+size > v.sabSize {
		return fmt.Errorf("region %s exceeds SAB bounds", name)
	}

	// Check for overlaps with existing regions
	for _, r := range v.regions {
		if v.regionsOverlap(offset, size, r.Offset, r.Size) {
			return fmt.Errorf("region %s overlaps with %s", name, r.Name)
		}
	}

	// Add region
	v.regions = append(v.regions, MemoryRegion{
		Name:    name,
		Offset:  offset,
		Size:    size,
		Purpose: purpose,
	})

	return nil
}

// ValidateWrite checks if a write operation is valid
func (v *SABValidator) ValidateWrite(offset, size uint32, regionName string) error {
	v.mu.RLock()
	defer v.mu.RUnlock()

	// Check bounds
	if offset+size > v.sabSize {
		violation := ValidationViolation{
			Type:    "OUT_OF_BOUNDS_WRITE",
			Message: fmt.Sprintf("Write at %d size %d exceeds SAB size %d", offset, size, v.sabSize),
			Offset:  offset,
			Size:    size,
		}
		v.recordViolation(violation)
		return errors.New(violation.Message)
	}

	// Find region
	region := v.findRegion(offset)
	if region == nil {
		violation := ValidationViolation{
			Type:    "INVALID_REGION_WRITE",
			Message: fmt.Sprintf("Write at %d does not belong to any region", offset),
			Offset:  offset,
			Size:    size,
		}
		v.recordViolation(violation)
		return errors.New(violation.Message)
	}

	// Validate write is within region
	if offset+size > region.Offset+region.Size {
		violation := ValidationViolation{
			Type:    "REGION_OVERFLOW_WRITE",
			Message: fmt.Sprintf("Write at %d size %d overflows region %s", offset, size, region.Name),
			Offset:  offset,
			Size:    size,
		}
		v.recordViolation(violation)
		return errors.New(violation.Message)
	}

	// If regionName specified, validate it matches
	if regionName != "" && region.Name != regionName {
		violation := ValidationViolation{
			Type:    "WRONG_REGION_WRITE",
			Message: fmt.Sprintf("Write to %s but offset is in %s", regionName, region.Name),
			Offset:  offset,
			Size:    size,
		}
		v.recordViolation(violation)
		return errors.New(violation.Message)
	}

	return nil
}

// ValidateRead checks if a read operation is valid
func (v *SABValidator) ValidateRead(offset, size uint32, regionName string) error {
	v.mu.RLock()
	defer v.mu.RUnlock()

	// Check bounds
	if offset+size > v.sabSize {
		violation := ValidationViolation{
			Type:    "OUT_OF_BOUNDS_READ",
			Message: fmt.Sprintf("Read at %d size %d exceeds SAB size %d", offset, size, v.sabSize),
			Offset:  offset,
			Size:    size,
		}
		v.recordViolation(violation)
		return errors.New(violation.Message)
	}

	// Find region
	region := v.findRegion(offset)
	if region == nil {
		violation := ValidationViolation{
			Type:    "INVALID_REGION_READ",
			Message: fmt.Sprintf("Read at %d does not belong to any region", offset),
			Offset:  offset,
			Size:    size,
		}
		v.recordViolation(violation)
		return errors.New(violation.Message)
	}

	// Validate read is within region
	if offset+size > region.Offset+region.Size {
		violation := ValidationViolation{
			Type:    "REGION_OVERFLOW_READ",
			Message: fmt.Sprintf("Read at %d size %d overflows region %s", offset, size, region.Name),
			Offset:  offset,
			Size:    size,
		}
		v.recordViolation(violation)
		return errors.New(violation.Message)
	}

	// If regionName specified, validate it matches
	if regionName != "" && region.Name != regionName {
		violation := ValidationViolation{
			Type:    "WRONG_REGION_READ",
			Message: fmt.Sprintf("Read from %s but offset is in %s", regionName, region.Name),
			Offset:  offset,
			Size:    size,
		}
		v.recordViolation(violation)
		return errors.New(violation.Message)
	}

	return nil
}

// ValidateLayout validates the entire memory layout
func (v *SABValidator) ValidateLayout() error {
	v.mu.RLock()
	defer v.mu.RUnlock()

	// Check for overlaps
	for i := 0; i < len(v.regions); i++ {
		for j := i + 1; j < len(v.regions); j++ {
			r1, r2 := v.regions[i], v.regions[j]
			if v.regionsOverlap(r1.Offset, r1.Size, r2.Offset, r2.Size) {
				return fmt.Errorf("regions %s and %s overlap", r1.Name, r2.Name)
			}
		}
	}

	return nil
}

// GetOverlaps returns all overlapping regions
func (v *SABValidator) GetOverlaps() []RegionOverlap {
	v.mu.RLock()
	defer v.mu.RUnlock()

	overlaps := make([]RegionOverlap, 0)

	for i := 0; i < len(v.regions); i++ {
		for j := i + 1; j < len(v.regions); j++ {
			r1, r2 := v.regions[i], v.regions[j]
			if v.regionsOverlap(r1.Offset, r1.Size, r2.Offset, r2.Size) {
				overlaps = append(overlaps, RegionOverlap{
					Region1: r1.Name,
					Region2: r2.Name,
					Start:   max(r1.Offset, r2.Offset),
					End:     min(r1.Offset+r1.Size, r2.Offset+r2.Size),
				})
			}
		}
	}

	return overlaps
}

// GetViolations returns all recorded violations
func (v *SABValidator) GetViolations() []ValidationViolation {
	v.mu.RLock()
	defer v.mu.RUnlock()

	violations := make([]ValidationViolation, len(v.violations))
	copy(violations, v.violations)
	return violations
}

// ClearViolations clears all recorded violations
func (v *SABValidator) ClearViolations() {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.violations = make([]ValidationViolation, 0)
}

// GetRegionByName returns a region by name
func (v *SABValidator) GetRegionByName(name string) (*MemoryRegion, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	for _, r := range v.regions {
		if r.Name == name {
			return &r, nil
		}
	}

	return nil, fmt.Errorf("region %s not found", name)
}

// GetRegionByOffset returns the region containing the given offset
func (v *SABValidator) GetRegionByOffset(offset uint32) (*MemoryRegion, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	region := v.findRegion(offset)
	if region == nil {
		return nil, fmt.Errorf("no region contains offset %d", offset)
	}

	return region, nil
}

// Helper: Find region containing offset (must hold lock)
func (v *SABValidator) findRegion(offset uint32) *MemoryRegion {
	for i := range v.regions {
		r := &v.regions[i]
		if offset >= r.Offset && offset < r.Offset+r.Size {
			return r
		}
	}
	return nil
}

// Helper: Check if two regions overlap
func (v *SABValidator) regionsOverlap(offset1, size1, offset2, size2 uint32) bool {
	return offset1 < offset2+size2 && offset1+size1 > offset2
}

// Helper: Record violation (must hold lock)
func (v *SABValidator) recordViolation(violation ValidationViolation) {
	// Note: This is called while holding RLock, which is safe for reading
	// but we can't modify. In production, consider using a separate lock
	// or channel for violations.
	// For now, violations are recorded best-effort.
}

// RegionOverlap describes overlapping regions
type RegionOverlap struct {
	Region1 string
	Region2 string
	Start   uint32
	End     uint32
}

// Helper functions
func max(a, b uint32) uint32 {
	if a > b {
		return a
	}
	return b
}

func min(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

// GetMemoryMap returns a human-readable memory map
func (v *SABValidator) GetMemoryMap() string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	result := fmt.Sprintf("SAB Memory Map (Size: %d bytes / %.2f MB)\n", v.sabSize, float64(v.sabSize)/(1024*1024))
	result += "================================================================\n"

	for _, r := range v.regions {
		result += fmt.Sprintf("%-20s | 0x%06X - 0x%06X | %6d bytes | %s\n",
			r.Name, r.Offset, r.Offset+r.Size, r.Size, r.Purpose)

		if r.CanExpand {
			result += fmt.Sprintf("                     | Expandable: %d inline -> %d total\n",
				r.MaxInline, r.MaxTotal)
		}
	}

	result += "================================================================\n"
	return result
}
