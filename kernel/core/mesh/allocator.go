package mesh

import (
	"math"
)

// AdaptiveAllocator calculates optimal replica counts for shared compute/storage
// Based on resource size, demand, and network capacity
type AdaptiveAllocator struct {
	minReplicas int     // Minimum replicas (default: 5)
	maxReplicas int     // Maximum replicas (default: 700)
	targetLoad  float64 // Target load per node (default: 0.375 = 37.5%)
	maxLoad     float64 // Maximum load per node (default: 0.50 = 50%)
}

// Resource represents a compute or storage resource
type Resource struct {
	Size         uint64  // Size in bytes
	Type         string  // "chunk", "model", "compute"
	DemandScore  float64 // 0.0 to 1.0, higher = more demand
	CreditBudget float64 // Available credits for replication
}

const (
	KB = 1024
	MB = 1024 * KB
	GB = 1024 * MB
)

// NewAdaptiveAllocator creates a new allocator
func NewAdaptiveAllocator(minReplicas, maxReplicas int, targetLoad, maxLoad float64) *AdaptiveAllocator {
	return &AdaptiveAllocator{
		minReplicas: minReplicas,
		maxReplicas: maxReplicas,
		targetLoad:  targetLoad,
		maxLoad:     maxLoad,
	}
}

// CalculateReplicas determines optimal replica count
func (aa *AdaptiveAllocator) CalculateReplicas(r Resource) int {
	// 1. Base replicas from size
	sizeReplicas := aa.replicasFromSize(r.Size)

	// 2. Adjust for demand
	demandMultiplier := 1.0 + r.DemandScore

	// 3. Adjust for budget (if provided)
	budgetMultiplier := aa.budgetMultiplier(r.CreditBudget)

	// 4. Calculate ideal replicas
	idealReplicas := int(float64(sizeReplicas) * demandMultiplier * budgetMultiplier)

	// 5. Clamp to min/max
	return clamp(idealReplicas, aa.minReplicas, aa.maxReplicas)
}

// replicasFromSize calculates base replicas based on resource size
func (aa *AdaptiveAllocator) replicasFromSize(size uint64) int {
	switch {
	case size < 1*MB:
		// Small files: 5-7 replicas
		return 5

	case size < 10*MB:
		// Medium files: 7-15 replicas
		return 7 + int(float64(size-1*MB)/float64(9*MB)*8)

	case size < 100*MB:
		// Large files: 15-50 replicas
		return 15 + int(float64(size-10*MB)/float64(90*MB)*35)

	case size < 1*GB:
		// Very large files: 50-150 replicas
		return 50 + int(float64(size-100*MB)/float64(900*MB)*100)

	case size < 10*GB:
		// Huge files: 150-500 replicas
		return 150 + int(float64(size-1*GB)/float64(9*GB)*350)

	default:
		// Massive files: 500-700 replicas
		remaining := float64(size - 10*GB)
		scale := math.Min(remaining/float64(90*GB), 1.0)
		return 500 + int(scale*200)
	}
}

// budgetMultiplier adjusts replicas based on available credits
func (aa *AdaptiveAllocator) budgetMultiplier(budget float64) float64 {
	if budget <= 0 {
		return 0.5 // Low budget = fewer replicas
	}
	if budget < 10 {
		return 0.7
	}
	if budget < 100 {
		return 1.0
	}
	if budget < 1000 {
		return 1.2
	}
	return 1.5 // High budget = more replicas
}

// CalculateChunkDistribution determines how to split a resource into chunks
func (aa *AdaptiveAllocator) CalculateChunkDistribution(totalSize uint64) ChunkDistribution {
	const optimalChunkSize = 1 * MB

	numChunks := int(math.Ceil(float64(totalSize) / float64(optimalChunkSize)))

	// For very large resources, use larger chunks
	chunkSize := optimalChunkSize
	if totalSize > 10*GB {
		chunkSize = 4 * MB
		numChunks = int(math.Ceil(float64(totalSize) / float64(chunkSize)))
	}

	return ChunkDistribution{
		TotalSize:     totalSize,
		ChunkSize:     uint64(chunkSize),
		NumChunks:     numChunks,
		LastChunkSize: uint64(totalSize) - uint64((numChunks-1)*int(chunkSize)),
	}
}

// ChunkDistribution describes how a resource is chunked
type ChunkDistribution struct {
	TotalSize     uint64
	ChunkSize     uint64
	NumChunks     int
	LastChunkSize uint64
}

// CalculateStorageCost estimates credit cost for storing a resource
func (aa *AdaptiveAllocator) CalculateStorageCost(r Resource, replicas int) float64 {
	// Base cost: 0.001 credits per MB per day
	baseCostPerMBPerDay := 0.001

	sizeMB := float64(r.Size) / float64(MB)
	dailyCost := sizeMB * baseCostPerMBPerDay * float64(replicas)

	// Adjust for demand (high demand = higher cost)
	demandMultiplier := 1.0 + r.DemandScore*0.5

	return dailyCost * demandMultiplier
}

// CalculateRetrievalCost estimates credit cost for fetching a resource
func (aa *AdaptiveAllocator) CalculateRetrievalCost(r Resource) float64 {
	// Base cost: 0.0001 credits per MB
	baseCostPerMB := 0.0001

	sizeMB := float64(r.Size) / float64(MB)
	return sizeMB * baseCostPerMB
}

// EstimateNetworkLoad estimates network load for a resource
func (aa *AdaptiveAllocator) EstimateNetworkLoad(r Resource, replicas int) NetworkLoad {
	distribution := aa.CalculateChunkDistribution(r.Size)

	// Estimate bandwidth per node
	avgBandwidthMbps := 10.0                                                         // Assume 10 Mbps average
	transferTimeSec := float64(distribution.ChunkSize) / (avgBandwidthMbps * 125000) // 125KB/s per Mbps

	return NetworkLoad{
		TotalBytes:       r.Size,
		BytesPerNode:     r.Size / uint64(replicas),
		EstimatedTimeSec: transferTimeSec * float64(distribution.NumChunks) / float64(replicas),
		BandwidthMbps:    avgBandwidthMbps * float64(replicas),
	}
}

// NetworkLoad describes estimated network usage
type NetworkLoad struct {
	TotalBytes       uint64
	BytesPerNode     uint64
	EstimatedTimeSec float64
	BandwidthMbps    float64
}

func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
