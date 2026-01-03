package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type ProjectContext struct {
	Project               string                 `json:"project"`
	Version               string                 `json:"version"`
	Philosophy            string                 `json:"philosophy"`
	InvestigationProtocol map[string]interface{} `json:"investigation_protocol"`
	Architecture          map[string]interface{} `json:"architecture"`
	Modules               map[string]interface{} `json:"modules"`
	Units                 map[string]interface{} `json:"units"`
	Protocols             map[string]interface{} `json:"protocols"`
	SearchKeywords        []string               `json:"search_keywords"`
	BuildSystem           map[string]interface{} `json:"build_system"`
}

func main() {
	root := "."
	ctx := ProjectContext{
		Project:    "INOS (Internet-Native Operating System)",
		Version:    "1.9-production",
		Philosophy: "SAB-Native, Zero-Copy, Epoch-Based Signaling, High-Performance Rust/Go WASM Hybrid.",
		InvestigationProtocol: map[string]interface{}{
			"core_principle": "Understand before modifying. The codebase is a living system with established patterns. Changes must respect architectural integrity.",
			"build_system_guidance": map[string]string{
				"primary_tool":         "make",
				"kernel_check":         "make kernel-test (Go tests) or make lint (Go vet)",
				"module_check_all":     "make modules-build (checks all)",
				"module_check_one":     "make check-module MODULE=<name> (e.g., make check-module MODULE=ml)",
				"module_test_one":      "make test-module MODULE=<name>",
				"full_system_build":    "make all",
				"frontend_integration": "make frontend-build (requires kernel/modules built)",
			},
			"phase_1_context_immersion": map[string]interface{}{
				"goal": "Achieve deep understanding of the affected system area",
				"required_actions": []string{
					"Run 'find . -name \"*.go\" -o -name \"*.rs\" -o -name \"*.js\" -o -name \"*.ts\" | xargs grep -l \"<relevant_terms>\"' to locate related code",
					"Create dependency map: trace imports/exports 3 levels deep",
					"Identify the established architectural patterns in this subsystem",
					"Document data flow: from source → transformations → destination",
					"Note any SAB (SharedArrayBuffer) usage patterns or zero-copy boundaries",
					"Review 'make help' output below to identify relevant build targets",
				},
				"deliverable": "Brief architecture diagram (textual or mental) showing components and their interactions",
			},
			"phase_2_pattern_recognition": map[string]interface{}{
				"goal": "Identify how similar problems are solved elsewhere",
				"required_actions": []string{
					"Search for established patterns: 'grep -r \"TODO: pattern\" .' or 'grep -r \"FIXME: convention\" .'",
					"Examine 2-3 similar functional areas for canonical implementations",
					"Check for existing tests that demonstrate expected behavior",
					"Look for configuration patterns in /config or /env directories",
					"Review protocol schemas (.capnp files) for data structure constraints",
				},
				"deliverable": "List of applicable patterns and their usage contexts",
			},
			"phase_3_root_cause_analysis": map[string]interface{}{
				"goal": "Pinpoint exact failure location and mechanism",
				"techniques": []string{
					"Binary elimination: Bisect the code path to isolate failure segment",
					"Data provenance: Trace specific data through transformations",
					"Epoch analysis: Check signaling boundaries and synchronization points",
					"Boundary inspection: Examine WASM/Rust/Go interop layers",
					"Resource flow: Verify memory ownership and zero-copy handoffs",
				},
				"validation_requirements": []string{
					"Must identify the exact line or function where behavior diverges",
					"Must verify hypothesis against 3+ test cases or examples",
					"Must check for similar issues in git history: 'git log --grep=\"<related>\"'",
					"Must consider cross-service impact in P2P mesh context",
				},
			},
			"phase_4_architectural_alignment_check": map[string]interface{}{
				"goal": "Ensure solution respects system constraints",
				"checklist": []string{
					"Does this change maintain zero-copy principles?",
					"Does it respect epoch-based signaling boundaries?",
					"Does it align with SAB memory ownership model?",
					"Will it affect P2P mesh gossip propagation?",
					"Does it match existing Rust/Go/WASM interop patterns?",
					"Are there similar fixes in the codebase to reference?",
					"Does it avoid creating new files when existing ones suffice?",
				},
				"required_references": []string{
					"Cite 2-3 existing examples of similar patterns",
					"Reference protocol schema definitions if data structures change",
					"Note any affected tests that need updating",
				},
			},
			"phase_5_solution_implementation": map[string]interface{}{
				"goal": "Apply minimal, focused changes",
				"principles": []string{
					"Change only what's necessary to fix root cause",
					"Preserve all existing interfaces unless absolutely required",
					"Follow the established code style and naming conventions",
					"Add or update tests to validate fix and prevent regression",
					"Document why this specific change solves the problem",
				},
				"verification_steps": []string{
					"Run 'make test' to verify all components",
					"Run 'make build' to verify compilation",
					"Run existing test suite",
					"Create minimal reproduction to verify fix",
					"Check for side effects in related subsystems",
					"Verify performance characteristics (zero-copy preserved)",
				},
			},
			"mandatory_investigation_commands": []string{
				"Before any edit: 'grep -r \"function_name\\|class_name\\|struct_name\" . --include=\"*.{go,rs,js,ts}\"'",
				"Examine callers: 'grep -r \"\\.function_name\\|-\u003efunction_name\" .'",
				"Check recent changes: 'git log -p --since=\"2 weeks ago\" -- path/to/file'",
				"Find similar patterns: 'grep -B5 -A5 \"pattern_to_match\" relevant_files'",
				"Verify imports/dependencies: 'grep -r \"import\\|use\\|require\" file.go | head -20'",
			},
			"architectural_constraints": map[string]interface{}{
				"non_negotiables": []string{
					"SAB memory model must be preserved",
					"Zero-copy boundaries cannot be broken without justification",
					"Epoch-based signaling must remain consistent",
					"P2P mesh must maintain eventual consistency",
					"WASM module boundaries must respect ownership",
				},
				"preferred_patterns": []string{
					"Use existing supervisor patterns for fault tolerance",
					"Follow StreamRPC conventions for service communication",
					"Adhere to gossip protocol for state propagation",
					"Use CRDT patterns for distributed state where applicable",
					"Follow knowledge graph structure for metadata",
				},
			},
			"deliverables": map[string]interface{}{
				"before_changes": []string{
					"Architecture reference showing affected area",
					"Pattern matches from elsewhere in codebase",
					"Root cause hypothesis with evidence",
					"Impact analysis on related subsystems",
				},
				"with_changes": []string{
					"Minimal diff focusing only on root cause",
					"Updated tests demonstrating fix",
					"Brief comment linking to architectural pattern used",
				},
			},
		},
		Architecture: map[string]interface{}{
			"Layer 1": map[string]interface{}{
				"name":         "Hybrid Host (Native)",
				"technologies": []string{"Nginx", "Brotli", "JavaScript (Web API Bridge)"},
				"components":   make(map[string]interface{}),
			},
			"Layer 2": map[string]interface{}{
				"name":         "Kernel (WASM)",
				"technologies": []string{"Go", "WASM"},
				"components":   make(map[string]interface{}),
			},
			"Layer 3": map[string]interface{}{
				"name":         "Modules (WASM)",
				"technologies": []string{"Rust", "JavaScript (React)"},
				"components":   make(map[string]interface{}),
			},
			"Foundation": map[string]interface{}{
				"name":       "Infrastructure SDK",
				"philosophy": "Zero-Copy Primitives & SAB Management",
				"components": make(map[string]interface{}),
			},
		},
		Modules:   make(map[string]interface{}),
		Units:     make(map[string]interface{}),
		Protocols: make(map[string]interface{}),
		SearchKeywords: []string{
			"SharedArrayBuffer", "Zero-Copy", "Epoch-Based Signaling", "WASM", "P2P Mesh",
			"Gossip Protocol", "Reputation Engine", "Knowledge Graph", "CRDT", "StreamRPC",
			"memory ownership", "synchronization primitive", "mesh coordination", "capnp schema",
		},
	}

	// 1. Parse unit_loader.go for Memory Addresses & Unit List
	loaderPath := "kernel/threads/unit_loader.go"
	if content, err := os.ReadFile(loaderPath); err == nil {
		s := string(content)
		reInbox := regexp.MustCompile(`Inbox:\s+(0x[0-9A-Fa-f]+)`)
		reOutbox := regexp.MustCompile(`Outbox:\s+(0x[0-9A-Fa-f]+)`)
		reEpoch := regexp.MustCompile(`Epoch:\s+Index\s+(\d+)`)

		inbox := "unknown"
		if m := reInbox.FindStringSubmatch(s); len(m) > 1 {
			inbox = m[1]
		}
		outbox := "unknown"
		if m := reOutbox.FindStringSubmatch(s); len(m) > 1 {
			outbox = m[1]
		}
		epoch := "unknown"
		if m := reEpoch.FindStringSubmatch(s); len(m) > 1 {
			epoch = m[1]
		}

		ctx.Architecture["Layer 2"].(map[string]interface{})["shared_bridge"] = map[string]string{
			"inbox":  inbox,
			"outbox": outbox,
			"epoch":  epoch,
			"source": loaderPath,
		}

		reUnits := regexp.MustCompile(`unitsList\s+:=\s+\[\]string\{([^}]+)\}`)
		if m := reUnits.FindStringSubmatch(s); len(m) > 1 {
			unitsRaw := strings.Split(m[1], ",")
			for _, u := range unitsRaw {
				u = strings.Trim(strings.TrimSpace(u), "\"")
				if u != "" {
					ctx.Units[u] = map[string]interface{}{
						"name": u,
						"id":   u,
					}
				}
			}
		}
	}

	// 2. Discover ALL Unit Implementations in Modules
	filepath.Walk("modules", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".rs") {
			return nil
		}
		// lib.rs is usually a module entry point
		if info.Name() == "mod.rs" || info.Name() == "lib.rs" {
			return nil
		}

		caps, uName := extractUnitInfoFromRust(path)
		if len(caps) > 0 {
			if uName == "" {
				uName = strings.TrimSuffix(info.Name(), ".rs")
			}

			// If already exists in Units (from unit_loader), merge. Otherwise create new.
			if _, ok := ctx.Units[uName]; !ok {
				ctx.Units[uName] = map[string]interface{}{
					"name": uName,
					"id":   uName,
				}
			}
			unit := ctx.Units[uName].(map[string]interface{})
			unit["implementation"] = path
			unit["type"] = "Rust Unit"
			existingCaps, _ := unit["capabilities"].([]string)
			unit["capabilities"] = deduplicate(append(existingCaps, caps...))
		}
		return nil
	})

	// 2.5 Discover Foundation & Architecture
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if name == "node_modules" || name == ".git" || name == "target" || name == "dist" || name == "gen" || name == "vendor" || name == ".gemini" {
				return filepath.SkipDir
			}

			// Foundation
			if strings.HasSuffix(path, "modules/sdk") {
				ctx.Architecture["Foundation"].(map[string]interface{})["components"].(map[string]interface{})[path] = map[string]interface{}{
					"path":         path,
					"capabilities": []string{"SAB Management", "Epoch Signaling", "Buddy Allocation", "State Conflict Resolution", "Zero-Copy IPC"},
				}
			}

			// Architecture Mapping (Layer 2)
			if strings.Contains(path, "kernel/") {
				layer2 := ctx.Architecture["Layer 2"].(map[string]interface{})["components"].(map[string]interface{})
				caps := detectKernelCapabilities(path)
				if len(caps) > 0 {
					layer2[path] = map[string]interface{}{"path": path, "capabilities": caps}
				}
			}

			// Modules Mapping (Layer 3)
			if strings.Contains(path, "modules/") && !strings.Contains(path, "modules/sdk") {
				if _, err := os.Stat(filepath.Join(path, "Cargo.toml")); err == nil {
					layer3 := ctx.Architecture["Layer 3"].(map[string]interface{})["components"].(map[string]interface{})
					modCaps := detectModuleCapabilities(path)
					ctx.Modules[path] = map[string]interface{}{"path": path, "type": "Rust Module", "capabilities": modCaps}
					layer3[path] = map[string]interface{}{"path": path, "capabilities": modCaps}
				}
			}
		}
		return nil
	})

	// 3. Supervisor Discovery & Go Capability Enrichment
	for name, unitRaw := range ctx.Units {
		unit := unitRaw.(map[string]interface{})

		supPath := filepath.Join("kernel/threads/supervisor/units", name+"_supervisor.go")
		if _, err := os.Stat(supPath); err == nil {
			unit["supervisor"] = supPath
			unit["controller"] = "Go Supervisor"

			if content, err := os.ReadFile(supPath); err == nil {
				reGoCap := regexp.MustCompile(`capabilities\s+=\s+\[\]string\{([^}]+)\}`)
				if m := reGoCap.FindStringSubmatch(string(content)); len(m) > 1 {
					existingCaps, _ := unit["capabilities"].([]string)
					for _, c := range strings.Split(m[1], ",") {
						c = strings.Trim(strings.TrimSpace(c), "\"")
						if c != "" {
							existingCaps = append(existingCaps, c)
						}
					}
					unit["capabilities"] = deduplicate(existingCaps)
				}
			}
		}
	}

	// 4. Final Protocols Pass
	filepath.Walk("protocols/schemas", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".capnp") {
			dir := filepath.Dir(path)
			if _, ok := ctx.Protocols[dir]; !ok {
				ctx.Protocols[dir] = map[string]interface{}{"path": dir, "schemas": []string{}, "layers": []string{}}
			}
			p := ctx.Protocols[dir].(map[string]interface{})
			p["schemas"] = append(p["schemas"].([]string), info.Name())
			p["layers"] = deduplicate(append(p["layers"].([]string), detectSchemaLayers([]string{info.Name()})...))
		}
		return nil
	})

	// 5. Parse Makefile for Build Context
	ctx.BuildSystem = make(map[string]interface{})
	if content, err := os.ReadFile("Makefile"); err == nil {
		lines := strings.Split(string(content), "\n")
		targets := make(map[string]string)

		// Simple parser: looks for "target:" followed by description in comments or echo
		for i, line := range lines {
			if strings.Contains(line, ":") && !strings.HasPrefix(line, ".") && !strings.Contains(line, "=") {
				parts := strings.Split(line, ":")
				targetName := strings.TrimSpace(parts[0])
				if targetName == "" || strings.Contains(targetName, "%") {
					continue
				}

				// Look for description in previous comments
				desc := "Build target"
				if i > 0 && strings.HasPrefix(lines[i-1], "#") {
					desc = strings.TrimSpace(strings.TrimPrefix(lines[i-1], "#"))
				} else {
					// Or look ahead for @echo
					for j := i + 1; j < i+3 && j < len(lines); j++ {
						if strings.Contains(lines[j], "@echo") {
							echoParts := strings.Split(lines[j], "\"")
							if len(echoParts) > 1 {
								desc = echoParts[1]
								break
							}
						}
					}
				}
				targets[targetName] = desc
			}
		}
		ctx.BuildSystem["targets"] = targets
		ctx.BuildSystem["file_path"] = "Makefile"
		ctx.BuildSystem["notes"] = "Use 'make <target>' to execute. For modules, use 'make check-module MODULE=<name>'."
	}

	output, _ := json.MarshalIndent(ctx, "", "  ")
	fmt.Println(string(output))
}

func extractUnitInfoFromRust(path string) ([]string, string) {
	capabilities := []string{}
	uName := ""
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, ""
	}
	s := string(content)

	// Extract Name
	reName := regexp.MustCompile(`fn\s+name\(&self\)\s+->\s+&(?:'static\s+)?str\s+\{\s*"([^"]+)"`)
	if m := reName.FindStringSubmatch(s); len(m) > 1 {
		uName = m[1]
	}

	// Source Beta: UnitProxy actions()
	reActions := regexp.MustCompile(`fn\s+actions\(&self\)\s+->\s+Vec<&str>\s+\{([^}]+)\}`)
	if m := reActions.FindStringSubmatch(s); len(m) > 1 {
		reStr := regexp.MustCompile(`"([^"]+)"`)
		matches := reStr.FindAllStringSubmatch(m[1], -1)
		for _, sm := range matches {
			capabilities = append(capabilities, sm[1])
		}
	}

	// Source Gamma: methods() discovery
	reMethods := regexp.MustCompile(`fn\s+methods\(&self\)\s+->\s+Vec<[^>]+>\s+\{([^}]+)\}`)
	if m := reMethods.FindStringSubmatch(s); len(m) > 1 {
		reStr := regexp.MustCompile(`"([^"]+)"`)
		matches := reStr.FindAllStringSubmatch(m[1], -1)
		for _, sm := range matches {
			capabilities = append(capabilities, sm[1])
		}
	}

	// Source Delta: Fallback function markers
	reExp := regexp.MustCompile(`//\s+Capability:\s+(.+)`)
	matches := reExp.FindAllStringSubmatch(s, -1)
	for _, m := range matches {
		capabilities = append(capabilities, strings.TrimSpace(m[1]))
	}

	return capabilities, uName
}

func detectKernelCapabilities(path string) []string {
	capabilities := []string{}
	files, _ := os.ReadDir(path)
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		name := f.Name()
		if strings.HasSuffix(name, "_supervisor.go") || name == "supervisor.go" {
			typeStr := "Generic"
			if strings.Contains(name, "_") {
				typeStr = strings.Title(strings.Split(name, "_")[0])
			}
			capabilities = append(capabilities, typeStr+" Supervisor")
		}
		if name == "transport.go" || name == "gossip.go" || name == "dht.go" {
			capabilities = append(capabilities, "P2P Mesh Networking")
		}
		if name == "mesh_coordinator.go" {
			capabilities = append(capabilities, "Mesh Mesh Coordination")
		}
		if strings.Contains(name, "engine.go") {
			capabilities = append(capabilities, "Intelligence Engine")
		}
		if name == "sab.go" || name == "arena.go" {
			capabilities = append(capabilities, "Zero-Copy Memory Management")
		}
		if name == "signal_loop.go" {
			capabilities = append(capabilities, "Reactive Epoch Signaling")
		}
	}
	return deduplicate(capabilities)
}

func detectModuleCapabilities(path string) []string {
	capabilities := []string{}
	cargoPath := filepath.Join(path, "Cargo.toml")
	if content, err := os.ReadFile(cargoPath); err == nil {
		s := string(content)
		if strings.Contains(s, "wgpu") {
			capabilities = append(capabilities, "GPU Acceleration")
		}
		if strings.Contains(s, "rapier3d") {
			capabilities = append(capabilities, "Physics Simulation")
		}
		if strings.Contains(s, "burn") || strings.Contains(s, "candle") {
			capabilities = append(capabilities, "AI Inference")
		}
		if strings.Contains(s, "sha2") || strings.Contains(s, "blake3") {
			capabilities = append(capabilities, "Proof-of-Work / Hashing")
		}
		if strings.Contains(s, "brotli") || strings.Contains(s, "lz4") || strings.Contains(s, "snap") {
			capabilities = append(capabilities, "Compression")
		}
		if strings.Contains(s, "automerge") {
			capabilities = append(capabilities, "CRDT State Sync")
		}
	}
	return deduplicate(capabilities)
}

func detectSchemaLayers(schemas []string) []string {
	layers := []string{}
	for _, s := range schemas {
		if strings.Contains(s, "syscall") || strings.Contains(s, "orchestration") {
			layers = append(layers, "Layer 2 (Kernel)")
		}
		if strings.Contains(s, "mesh") || strings.Contains(s, "gossip") {
			layers = append(layers, "P2P Mesh")
		}
		if strings.Contains(s, "capsule") || strings.Contains(s, "model") || strings.Contains(s, "science") {
			layers = append(layers, "Layer 3 (Modules)")
		}
	}
	return deduplicate(layers)
}

func deduplicate(list []string) []string {
	unique := make(map[string]bool)
	result := []string{}
	for _, c := range list {
		if !unique[c] {
			unique[c] = true
			result = append(result, c)
		}
	}
	return result
}
