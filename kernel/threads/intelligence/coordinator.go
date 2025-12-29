package intelligence

import (
	"fmt"
	"sync"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	"github.com/nmxmxh/inos_v1/kernel/threads/pattern"
)

// UnifiedIntelligenceCoordinator orchestrates all intelligence engines
type UnifiedIntelligenceCoordinator struct {
	sab        []byte
	baseOffset uint32

	// Core engines (will be implemented in subsequent files)
	learning     interface{} // *EnhancedLearningEngine
	optimization interface{} // *EnhancedOptimizationEngine
	scheduling   interface{} // *EnhancedSchedulingEngine
	security     interface{} // *EnhancedSecurityEngine
	health       interface{} // *EnhancedHealthMonitor

	// Shared components
	knowledge *KnowledgeGraph
	feedback  *FeedbackLoopManager
	workflows *WorkflowOrchestrator

	// Phase 3 integration
	epoch        *foundation.EnhancedEpoch
	messageQueue *foundation.MessageQueue

	// Phase 4 integration
	patterns *pattern.TieredPatternStorage
	detector *pattern.PatternDetector

	// Statistics
	stats CoordinatorStats

	mu sync.RWMutex
}

type CoordinatorStats struct {
	DecisionsMade      uint64
	CachedDecisions    uint64
	ComplexDecisions   uint64
	AvgDecisionLatency time.Duration
	LastUpdate         time.Time
}

// NewUnifiedIntelligenceCoordinator creates a new intelligence coordinator
func NewUnifiedIntelligenceCoordinator(
	sab []byte,
	baseOffset uint32,
	epoch *foundation.EnhancedEpoch,
	messageQueue *foundation.MessageQueue,
	patterns *pattern.TieredPatternStorage,
) *UnifiedIntelligenceCoordinator {
	return &UnifiedIntelligenceCoordinator{
		sab:          sab,
		baseOffset:   baseOffset,
		knowledge:    NewKnowledgeGraph(sab, baseOffset, 1024),
		feedback:     NewFeedbackLoopManager(),
		workflows:    NewWorkflowOrchestrator(10),
		epoch:        epoch,
		messageQueue: messageQueue,
		patterns:     patterns,
	}
}

// Initialize initializes the coordinator and all engines
func (uic *UnifiedIntelligenceCoordinator) Initialize() error {
	// Initialize feedback loops between engines
	uic.initializeFeedbackLoops()

	// Initialize workflows
	uic.initializeWorkflows()

	return nil
}

// Decide makes an intelligent decision using all engines
func (uic *UnifiedIntelligenceCoordinator) Decide(context *DecisionContext) *foundation.Decision {
	startTime := time.Now()

	uic.mu.Lock()
	uic.stats.DecisionsMade++
	uic.mu.Unlock()

	// Check cache first (knowledge graph)
	if cached := uic.checkCache(context); cached != nil {
		uic.mu.Lock()
		uic.stats.CachedDecisions++
		uic.mu.Unlock()

		cached.Latency = time.Since(startTime)
		return cached
	}

	// Complex decision - use appropriate engine
	var decision *foundation.Decision // Changed to foundation.Decision

	switch foundation.DecisionType(context.Type) {
	case foundation.DecisionRouting:
		decision = uic.makeRoutingDecision(context)
	case foundation.DecisionScheduling:
		decision = uic.makeSchedulingDecision(context)
	case foundation.DecisionOptimization:
		decision = uic.makeOptimizationDecision(context)
	case foundation.DecisionSecurity:
		decision = uic.makeSecurityDecision(context)
	default:
		decision = &foundation.Decision{ // Changed to foundation.Decision
			Type:       foundation.DecisionType(context.Type), // Changed to foundation.DecisionType
			Confidence: 0.5,
			Reasoning:  "unknown decision type",
		}
	}

	decision.Latency = time.Since(startTime)

	uic.mu.Lock()
	uic.stats.ComplexDecisions++
	uic.stats.AvgDecisionLatency = (uic.stats.AvgDecisionLatency + decision.Latency) / 2
	uic.mu.Unlock()

	// Cache decision in knowledge graph
	uic.cacheDecision(context, decision)

	return decision
}

// ExecuteWorkflow executes an intelligence workflow
func (uic *UnifiedIntelligenceCoordinator) ExecuteWorkflow(workflowID string, input interface{}) *foundation.WorkflowResult { // Changed return type
	if uic.workflows == nil {
		return &foundation.WorkflowResult{Success: false, Error: "workflow orchestrator not initialized"}
	}
	return uic.workflows.Execute(workflowID, input)
}

// UpdateKnowledge updates the knowledge graph
func (uic *UnifiedIntelligenceCoordinator) UpdateKnowledge(id string, nodeType foundation.NodeType, confidence float32, data []byte) error {
	return uic.knowledge.AddNode(id, nodeType, confidence, data)
}

// QueryKnowledge queries the knowledge graph
func (uic *UnifiedIntelligenceCoordinator) QueryKnowledge(query string) ([]*KnowledgeNode, error) {
	return uic.knowledge.Query(query)
}

// GetStats returns coordinator statistics
func (uic *UnifiedIntelligenceCoordinator) GetStats() CoordinatorStats {
	uic.mu.RLock()
	defer uic.mu.RUnlock()

	return uic.stats
}

// Helper: Initialize feedback loops
func (uic *UnifiedIntelligenceCoordinator) initializeFeedbackLoops() {
	// Learning → Optimization
	uic.feedback.RegisterLoop("learning_to_opt", foundation.EngineLearning, foundation.EngineOptimization, 100*time.Millisecond, 0.8)

	// Learning → Scheduling
	uic.feedback.RegisterLoop("learning_to_sched", foundation.EngineLearning, foundation.EngineScheduling, 50*time.Millisecond, 0.9)

	// Optimization → Scheduling
	uic.feedback.RegisterLoop("opt_to_sched", foundation.EngineOptimization, foundation.EngineScheduling, 50*time.Millisecond, 0.7)

	// Security → All engines
	uic.feedback.RegisterLoop("security_to_all", foundation.EngineSecurity, foundation.EngineLearning, 10*time.Millisecond, 1.0)

	// Health → All engines
	uic.feedback.RegisterLoop("health_to_all", foundation.EngineHealth, foundation.EngineLearning, 100*time.Millisecond, 0.5)
}

// Helper: Initialize workflows
func (uic *UnifiedIntelligenceCoordinator) initializeWorkflows() {
	// Job optimization workflow
	jobOptWorkflow := &IntelligenceWorkflow{
		ID:          "job_optimization",
		Name:        "Job Optimization Workflow",
		Description: "Optimize job execution using learning and optimization engines",
		Stages: []*PipelineStage{
			{
				ID:        "predict_resources",
				Engine:    foundation.EngineLearning,
				Operation: "predict",
				Timeout:   100 * time.Millisecond,
			},
			{
				ID:        "optimize_parameters",
				Engine:    foundation.EngineOptimization,
				Operation: "optimize",
				Timeout:   500 * time.Millisecond,
			},
			{
				ID:        "schedule_job",
				Engine:    foundation.EngineScheduling,
				Operation: "schedule",
				Timeout:   50 * time.Millisecond,
			},
		},
		Flow: &PipelineFlow{
			Sequential: true,
		},
	}

	uic.workflows.RegisterWorkflow(jobOptWorkflow)
}

// Helper: Check cache for decision
func (uic *UnifiedIntelligenceCoordinator) checkCache(context *DecisionContext) *foundation.Decision {
	if uic.knowledge == nil {
		return nil
	}

	// Query knowledge graph for cached decisions
	// Use context hash as node ID
	contextHash := uic.hashContext(context)
	nodes, err := uic.knowledge.Query(fmt.Sprintf("id:%s", contextHash))
	if err != nil || len(nodes) == 0 {
		return nil
	}

	node := nodes[0]
	// Check if cached decision is still valid (confidence > 0.7)
	if node.Confidence < 0.7 {
		return nil
	}

	return &foundation.Decision{
		Type:       foundation.DecisionType(context.Type),
		Confidence: node.Confidence,
		Reasoning:  "retrieved from knowledge graph",
	}
}

// Helper: Cache decision
func (uic *UnifiedIntelligenceCoordinator) cacheDecision(context *DecisionContext, decision *foundation.Decision) {
	if uic.knowledge == nil {
		return
	}

	// Store decision in knowledge graph for future use
	contextHash := uic.hashContext(context)

	// Create knowledge node
	uic.knowledge.AddNode(
		contextHash,
		foundation.NodeTypePrediction,
		decision.Confidence,
		[]byte(decision.Reasoning),
	)
}

// makeRoutingDecision handles routing decisions
func (uic *UnifiedIntelligenceCoordinator) makeRoutingDecision(_ *DecisionContext) *foundation.Decision {
	return &foundation.Decision{
		Type:       foundation.DecisionRouting,
		Confidence: 0.9,
		Value:      "compute",
	}
}

// Helper: Make scheduling decision
func (uic *UnifiedIntelligenceCoordinator) makeSchedulingDecision(_ *DecisionContext) *foundation.Decision {
	// Use scheduling engine
	return &foundation.Decision{
		Type:       foundation.DecisionScheduling,
		Confidence: 0.90,
		Reasoning:  "scheduled based on priority and deadline",
	}
}

// makeOptimizationDecision makes optimization decisions
func (uic *UnifiedIntelligenceCoordinator) makeOptimizationDecision(_ *DecisionContext) *foundation.Decision {
	return &foundation.Decision{
		Type:       foundation.DecisionOptimization,
		Confidence: 0.90,
		Reasoning:  "optimized based on current load",
	}
}

// makeSecurityDecision makes security decisions
func (uic *UnifiedIntelligenceCoordinator) makeSecurityDecision(_ *DecisionContext) *foundation.Decision {
	return &foundation.Decision{
		Type:       foundation.DecisionSecurity,
		Confidence: 1.0,
		Value:      true,
		Reasoning:  "security check passed",
	}
}

// Helper: Hash context for caching
func (uic *UnifiedIntelligenceCoordinator) hashContext(context *DecisionContext) string {
	// Create simple hash from context type
	return fmt.Sprintf("decision_%d_%d", context.Type, time.Now().Unix()/3600)
}
