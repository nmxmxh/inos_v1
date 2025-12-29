package scheduling

import (
	"sync"
	"time"
)

// ResourceAllocator implements bin packing for resource allocation
type ResourceAllocator struct {
	nodes     []*ResourceNode
	algorithm AllocationAlgorithm
	mu        sync.RWMutex
}

type ResourceNode struct {
	ID        string
	Available ResourceRequirements
	Capacity  ResourceRequirements
	Jobs      []string
}

type ResourceAllocation struct {
	NodeID    string
	Allocated ResourceRequirements
	Success   bool
}

type AllocationAlgorithm int

const (
	FirstFit AllocationAlgorithm = iota
	BestFit
	WorstFit
)

func NewResourceAllocator() *ResourceAllocator {
	return &ResourceAllocator{
		nodes:     make([]*ResourceNode, 0),
		algorithm: BestFit,
	}
}

// Allocate allocates resources using bin packing
func (ra *ResourceAllocator) Allocate(requirements ResourceRequirements) *ResourceAllocation {
	ra.mu.Lock()
	defer ra.mu.Unlock()

	// Find suitable node based on algorithm
	var selectedNode *ResourceNode

	switch ra.algorithm {
	case FirstFit:
		selectedNode = ra.firstFit(requirements)
	case BestFit:
		selectedNode = ra.bestFit(requirements)
	case WorstFit:
		selectedNode = ra.worstFit(requirements)
	}

	if selectedNode == nil {
		return &ResourceAllocation{
			Success: false,
		}
	}

	// Allocate resources
	selectedNode.Available.CPU -= requirements.CPU
	selectedNode.Available.Memory -= requirements.Memory
	selectedNode.Available.GPU -= requirements.GPU

	return &ResourceAllocation{
		NodeID:    selectedNode.ID,
		Allocated: requirements,
		Success:   true,
	}
}

// First Fit: allocate to first node that fits
func (ra *ResourceAllocator) firstFit(req ResourceRequirements) *ResourceNode {
	for _, node := range ra.nodes {
		if ra.canFit(node, req) {
			return node
		}
	}
	return nil
}

// Best Fit: allocate to node with least remaining resources after allocation
func (ra *ResourceAllocator) bestFit(req ResourceRequirements) *ResourceNode {
	var bestNode *ResourceNode
	bestScore := -1.0

	for _, node := range ra.nodes {
		if !ra.canFit(node, req) {
			continue
		}

		// Calculate remaining resources after allocation
		remaining := ResourceRequirements{
			CPU:    node.Available.CPU - req.CPU,
			Memory: node.Available.Memory - req.Memory,
			GPU:    node.Available.GPU - req.GPU,
		}

		// Score = total remaining (lower is better for best fit)
		score := remaining.CPU + float64(remaining.Memory)/(1024*1024*1024) + remaining.GPU

		if bestNode == nil || score < bestScore {
			bestNode = node
			bestScore = score
		}
	}

	return bestNode
}

// Worst Fit: allocate to node with most remaining resources
func (ra *ResourceAllocator) worstFit(req ResourceRequirements) *ResourceNode {
	var worstNode *ResourceNode
	worstScore := -1.0

	for _, node := range ra.nodes {
		if !ra.canFit(node, req) {
			continue
		}

		// Score = total available (higher is better for worst fit)
		score := node.Available.CPU + float64(node.Available.Memory)/(1024*1024*1024) + node.Available.GPU

		if worstNode == nil || score > worstScore {
			worstNode = node
			worstScore = score
		}
	}

	return worstNode
}

// Check if node can fit requirements
func (ra *ResourceAllocator) canFit(node *ResourceNode, req ResourceRequirements) bool {
	return node.Available.CPU >= req.CPU &&
		node.Available.Memory >= req.Memory &&
		node.Available.GPU >= req.GPU
}

// AddNode adds a resource node
func (ra *ResourceAllocator) AddNode(node *ResourceNode) {
	ra.mu.Lock()
	defer ra.mu.Unlock()

	ra.nodes = append(ra.nodes, node)
}

// Release releases allocated resources
func (ra *ResourceAllocator) Release(nodeID string, resources ResourceRequirements) {
	ra.mu.Lock()
	defer ra.mu.Unlock()

	for _, node := range ra.nodes {
		if node.ID == nodeID {
			node.Available.CPU += resources.CPU
			node.Available.Memory += resources.Memory
			node.Available.GPU += resources.GPU
			break
		}
	}
}

// DAGExecutor executes DAG workflows
type DAGExecutor struct {
	mu sync.Mutex
}

type DAGSchedule struct {
	Stages        [][]*Job // Jobs grouped by execution stage
	CriticalPath  []*Job
	TotalDuration time.Duration
}

func NewDAGExecutor() *DAGExecutor {
	return &DAGExecutor{}
}

// Schedule creates execution schedule for DAG
func (de *DAGExecutor) Schedule(jobs []*Job) *DAGSchedule {
	de.mu.Lock()
	defer de.mu.Unlock()

	// Build dependency graph
	graph := de.buildGraph(jobs)

	// Topological sort to find execution order
	stages := de.topologicalSort(graph)

	// Find critical path
	criticalPath := de.findCriticalPath(graph, stages)

	// Calculate total duration
	totalDuration := de.calculateDuration(criticalPath)

	return &DAGSchedule{
		Stages:        stages,
		CriticalPath:  criticalPath,
		TotalDuration: totalDuration,
	}
}

// Build dependency graph
func (de *DAGExecutor) buildGraph(jobs []*Job) map[string]*DAGNode {
	graph := make(map[string]*DAGNode)

	// Create nodes
	for _, job := range jobs {
		graph[job.ID] = &DAGNode{
			Job:      job,
			Children: make([]*DAGNode, 0),
			Parents:  make([]*DAGNode, 0),
		}
	}

	// Add edges
	for _, job := range jobs {
		node := graph[job.ID]
		for _, depID := range job.Dependencies {
			if parent, exists := graph[depID]; exists {
				node.Parents = append(node.Parents, parent)
				parent.Children = append(parent.Children, node)
			}
		}
	}

	return graph
}

// Topological sort for parallel execution stages
func (de *DAGExecutor) topologicalSort(graph map[string]*DAGNode) [][]*Job {
	stages := make([][]*Job, 0)
	inDegree := make(map[string]int)

	// Calculate in-degrees
	for id, node := range graph {
		inDegree[id] = len(node.Parents)
	}

	// Process stages
	for len(inDegree) > 0 {
		stage := make([]*Job, 0)

		// Find nodes with in-degree 0
		for id, degree := range inDegree {
			if degree == 0 {
				stage = append(stage, graph[id].Job)
			}
		}

		if len(stage) == 0 {
			break // Cycle detected or done
		}

		stages = append(stages, stage)

		// Remove processed nodes and update in-degrees
		for _, job := range stage {
			delete(inDegree, job.ID)

			for _, child := range graph[job.ID].Children {
				inDegree[child.Job.ID]--
			}
		}
	}

	return stages
}

// Find critical path (longest path)
func (de *DAGExecutor) findCriticalPath(graph map[string]*DAGNode, stages [][]*Job) []*Job {
	// Calculate earliest start times
	earliestStart := make(map[string]time.Duration)

	for _, stage := range stages {
		for _, job := range stage {
			maxParentTime := time.Duration(0)

			for _, parent := range graph[job.ID].Parents {
				parentEnd := earliestStart[parent.Job.ID] + parent.Job.Duration
				if parentEnd > maxParentTime {
					maxParentTime = parentEnd
				}
			}

			earliestStart[job.ID] = maxParentTime
		}
	}

	// Find job with maximum end time
	var lastJob *Job
	maxEndTime := time.Duration(0)

	for id, node := range graph {
		endTime := earliestStart[id] + node.Job.Duration
		if endTime > maxEndTime {
			maxEndTime = endTime
			lastJob = node.Job
		}
	}

	// Backtrack to find critical path
	path := make([]*Job, 0)
	current := lastJob

	for current != nil {
		path = append([]*Job{current}, path...)

		// Find parent on critical path
		var criticalParent *Job
		maxTime := time.Duration(0)

		for _, parent := range graph[current.ID].Parents {
			parentEnd := earliestStart[parent.Job.ID] + parent.Job.Duration
			if parentEnd > maxTime {
				maxTime = parentEnd
				criticalParent = parent.Job
			}
		}

		current = criticalParent
	}

	return path
}

// Calculate total duration
func (de *DAGExecutor) calculateDuration(path []*Job) time.Duration {
	total := time.Duration(0)
	for _, job := range path {
		total += job.Duration
	}
	return total
}

type DAGNode struct {
	Job      *Job
	Children []*DAGNode
	Parents  []*DAGNode
}
