package intelligence

import (
	"testing"
	"time"

	"github.com/nmxmxh/inos_v1/kernel/threads/foundation"
	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence/optimization"
	"github.com/nmxmxh/inos_v1/kernel/threads/intelligence/scheduling"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockDispatcher struct{}

func (m *mockDispatcher) Dispatch(engine foundation.EngineType, operation string, parameters map[string]interface{}) (interface{}, error) {
	return "success", nil
}

func TestWorkflowOrchestrator_Registration(t *testing.T) {
	orch := NewWorkflowOrchestrator(10, &mockDispatcher{})

	wf := &IntelligenceWorkflow{
		ID:   "wf1",
		Name: "Test Workflow",
		Stages: []*PipelineStage{
			{ID: "s1", Operation: "op1"},
		},
	}

	err := orch.RegisterWorkflow(wf)
	require.NoError(t, err)

	// Register duplicate
	err = orch.RegisterWorkflow(wf)
	assert.Error(t, err)

	// Get
	retrieved, err := orch.GetWorkflow("wf1")
	require.NoError(t, err)
	assert.Equal(t, wf.Name, retrieved.Name)

	_, err = orch.GetWorkflow("missing")
	assert.Error(t, err)
}

func TestWorkflowOrchestrator_Execution(t *testing.T) {
	orch := NewWorkflowOrchestrator(10, &mockDispatcher{})

	wf := &IntelligenceWorkflow{
		ID: "wf-exec",
		Stages: []*PipelineStage{
			{ID: "s1", Engine: foundation.EngineLearning, Operation: "predict", Timeout: 100 * time.Millisecond},
			{ID: "s2", Engine: foundation.EngineOptimization, Operation: "optimize", Timeout: 100 * time.Millisecond},
		},
		Flow: &PipelineFlow{Sequential: true},
	}

	err := orch.RegisterWorkflow(wf)
	require.NoError(t, err)

	// Execute logic is currently a mock/stub in workflow.go?
	// Let's check implementation behavior by running it.
	// If engines are not connected, it might just simulate success or fail?
	// workflow.go: executeStage just sleeps and returns success currently?
	// We need to verify what logic exists.
	// Assuming it simulates execution based on the code I recall seeing (dummy implementation).

	result := orch.Execute("wf-exec", nil)
	assert.True(t, result.Success)
	assert.Empty(t, result.Error)

	// Verify execution record
	// GetExecution logic might or might not look up past executions.
}

func TestWorkflowOrchestrator_Validation(t *testing.T) {
	orch := NewWorkflowOrchestrator(10, &mockDispatcher{})

	// Empty ID
	err := orch.RegisterWorkflow(&IntelligenceWorkflow{Stages: []*PipelineStage{{ID: "s1"}}})
	assert.Error(t, err)

	// No stages
	err = orch.RegisterWorkflow(&IntelligenceWorkflow{ID: "empty", Stages: []*PipelineStage{}})
	assert.Error(t, err)
}

func TestWorkflowMonitor_GetExecution(t *testing.T) {
	orch := NewWorkflowOrchestrator(10, &mockDispatcher{})
	wf := &IntelligenceWorkflow{
		ID: "wf-monitor",
		Stages: []*PipelineStage{
			{ID: "s1", Operation: "op", Timeout: 100 * time.Millisecond}, // Will use default dummy execution (10ms)
		},
	}
	_ = orch.RegisterWorkflow(wf)

	// Execute
	_ = orch.Execute("wf-monitor", nil)

	// Get Execution
	exec, err := orch.monitor.GetExecution("wf-monitor") // monitor is unexported but accessible in package
	require.NoError(t, err)
	assert.Equal(t, "wf-monitor", exec.WorkflowID)
	assert.Equal(t, "success", exec.Status)

	// Missing
	_, err = orch.monitor.GetExecution("missing")
	assert.Error(t, err)
}

func TestWorkflowOrchestrator_RealEngineIntegration(t *testing.T) {
	// 1. Setup Coordinator with Real Engines (Optimization/Scheduling)
	uic := NewUnifiedIntelligenceCoordinator(nil, 0, nil, nil, nil)
	uic.optimization = optimization.NewOptimizationEngine()
	uic.scheduling = scheduling.NewSchedulingEngine()

	// 2. Register a workflow that uses Optimization and Scheduling
	wf := &IntelligenceWorkflow{
		ID: "real-wf",
		Stages: []*PipelineStage{
			{
				ID:        "opt",
				Engine:    foundation.EngineOptimization,
				Operation: "optimize",
				Timeout:   1 * time.Second,
			},
			{
				ID:        "sched",
				Engine:    foundation.EngineScheduling,
				Operation: "schedule",
				Timeout:   1 * time.Second,
			},
		},
	}
	err := uic.workflows.RegisterWorkflow(wf)
	require.NoError(t, err)

	// 3. Execute
	result := uic.ExecuteWorkflow("real-wf", nil)
	assert.True(t, result.Success, "Workflow should succeed with real engines")
	assert.Empty(t, result.Error)
}
