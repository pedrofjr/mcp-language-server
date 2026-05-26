package main

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/mark3labs/mcp-go/mcp"
)

func TestNFRGates_CriticalHandlerTimeoutsMax15Seconds(t *testing.T) {
	if runQueryHandlerTimeout != NFRCriticalOperationTimeoutMax {
		t.Fatalf("runQueryHandlerTimeout=%s, want %s", runQueryHandlerTimeout, NFRCriticalOperationTimeoutMax)
	}
	if definitionReferencesHandlerTimeout != NFRCriticalOperationTimeoutMax {
		t.Fatalf("definitionReferencesHandlerTimeout=%s, want %s", definitionReferencesHandlerTimeout, NFRCriticalOperationTimeoutMax)
	}
}

func TestNFRGates_RepresentativeDelphiCorpusSuite_P95Under5Seconds(t *testing.T) {
	corpusRoot, err := resolveNFRCorpusRoot()
	if err != nil {
		t.Fatalf("resolveNFRCorpusRoot: %v", err)
	}
	stats, err := describeNFRCorpus(corpusRoot)
	if err != nil {
		t.Fatalf("describeNFRCorpus: %v", err)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})
	if err := os.Chdir(corpusRoot); err != nil {
		t.Fatalf("chdir corpus: %v", err)
	}

	sampleFile := stats.SampleFiles[0]
	svc := newNFRCorpusMCPServer(t, corpusRoot, sampleFile, 50*time.Millisecond)
	initializeTestMCPServer(t, svc)

	sampleURI := string(protocol.URIFromPath(sampleFile))
	loadCases := []struct {
		tool string
		args map[string]any
	}{
		{
			tool: "run_query",
			args: map[string]any{
				"node_type": "identifier",
				"limit":     float64(20),
			},
		},
		{
			tool: "diagnostics",
			args: map[string]any{
				"filePath":        sampleFile,
				"contextLines":    false,
				"showLineNumbers": false,
			},
		},
		{
			tool: "semantic_search",
			args: map[string]any{
				"query": "procedure",
				"limit": float64(10),
			},
		},
		{
			tool: "graph_query",
			args: map[string]any{
				"uri":       sampleURI,
				"direction": "both",
				"depth":     float64(1),
			},
		},
	}

	const samplesPerTool = 4
	var durations []time.Duration
	requestID := 9100

	for _, tc := range loadCases {
		for sample := 0; sample < samplesPerTool; sample++ {
			requestID++
			started := time.Now()
			resp := handleTestMCPRequest(
				t,
				svc,
				mcp.MethodToolsCall,
				map[string]any{
					"name":      tc.tool,
					"arguments": tc.args,
				},
				requestID,
			)
			elapsed := time.Since(started)
			durations = append(durations, elapsed)

			resultBytes, err := json.Marshal(resp.Result)
			if err != nil {
				t.Fatalf("%s sample=%d: marshal result: %v", tc.tool, sample, err)
			}
			var callResult map[string]any
			if err := json.Unmarshal(resultBytes, &callResult); err != nil {
				t.Fatalf("%s sample=%d: decode result: %v", tc.tool, sample, err)
			}
			if isError, _ := callResult["isError"].(bool); isError {
				t.Fatalf("%s sample=%d: expected success in Delphi corpus suite, got %s", tc.tool, sample, string(resultBytes))
			}
		}
	}

	p95 := nfrPercentileDuration(durations, 95)
	t.Log(formatNFRCorpusReport(stats, len(durations), p95.String()))

	if p95 > NFRRepresentativeLoadP95Max {
		t.Fatalf(
			"Delphi representative corpus p95=%s exceeds gate %s (corpus_files=%d samples=%d)",
			p95,
			NFRRepresentativeLoadP95Max,
			stats.DelphiFileCount,
			len(durations),
		)
	}
}

func newNFRCorpusMCPServer(t *testing.T, corpusRoot, samplePAS string, delay time.Duration) *mcpServer {
	t.Helper()

	client := newRegisterToolsRequestContextFakeLSPClientWithDelay(t, corpusRoot, samplePAS, delay)

	svc := &mcpServer{
		ctx:       context.Background(),
		lspClient: client,
		config:    config{workspaceDir: corpusRoot},
	}
	svc.mcpServer = newMCPServer(corpusRoot)

	if err := svc.registerTools(); err != nil {
		t.Fatalf("registerTools() returned error: %v", err)
	}

	return svc
}

func TestNFRGates_RepresentativeFakeLSPLoadSuite_P95Under5Seconds(t *testing.T) {
	svc := newRegisteredTestMCPServerWithContextFakeLSPDelay(t, 100*time.Millisecond)
	initializeTestMCPServer(t, svc)

	loadCases := []struct {
		tool string
		args map[string]any
	}{
		{
			tool: "definition",
			args: map[string]any{"symbolName": "TargetSymbol"},
		},
		{
			tool: "references",
			args: map[string]any{"symbolName": "TargetSymbol"},
		},
	}

	const samplesPerTool = 12
	var durations []time.Duration
	requestID := 9000

	for _, tc := range loadCases {
		for sample := 0; sample < samplesPerTool; sample++ {
			requestID++
			started := time.Now()
			resp := handleTestMCPRequest(
				t,
				svc,
				mcp.MethodToolsCall,
				map[string]any{
					"name":      tc.tool,
					"arguments": tc.args,
				},
				requestID,
			)
			elapsed := time.Since(started)
			durations = append(durations, elapsed)

			resultBytes, err := json.Marshal(resp.Result)
			if err != nil {
				t.Fatalf("%s sample=%d: marshal result: %v", tc.tool, sample, err)
			}
			var callResult map[string]any
			if err := json.Unmarshal(resultBytes, &callResult); err != nil {
				t.Fatalf("%s sample=%d: decode result: %v", tc.tool, sample, err)
			}
			if isError, _ := callResult["isError"].(bool); isError {
				t.Fatalf("%s sample=%d: expected success in representative load suite, got %s", tc.tool, sample, string(resultBytes))
			}
		}
	}

	p95 := nfrPercentileDuration(durations, 95)
	if p95 > NFRRepresentativeLoadP95Max {
		t.Fatalf("representative load p95=%s exceeds gate %s (samples=%d)", p95, NFRRepresentativeLoadP95Max, len(durations))
	}
}

func TestNFRGates_CriticalToolsOperationalErrorCoverageAtLeast95Percent(t *testing.T) {
	svc := newRegisteredTestMCPServer(t)
	initializeTestMCPServer(t, svc)

	fixturePath := nfrErrorFixturePath(t, svc)
	scenarios := inventoryOperationalErrorScenarios(fixturePath)
	if len(scenarios) == 0 {
		t.Fatal("expected at least one operational error scenario from inventory")
	}

	compliant := 0
	requestID := 8000

	for _, scenario := range scenarios {
		requestID++
		svcForScenario := svc
		if scenario.RequiresFakeLSP {
			svcForScenario = newRegisteredTestMCPServerWithContextFakeLSPDelay(t, 0)
			initializeTestMCPServer(t, svcForScenario)
		}

		resp := handleTestMCPRequest(
			t,
			svcForScenario,
			mcp.MethodToolsCall,
			map[string]any{
				"name":      scenario.Tool,
				"arguments": scenario.Args,
			},
			requestID,
		)

		payload, isError := toolCallResultPayload(t, resp)
		if !isError {
			t.Fatalf("%s (%s): expected error payload for operational audit, got %s", scenario.Tool, scenario.Kind, payload)
		}
		if isOperationalMCPErrorPayload(payload) {
			compliant++
		} else {
			t.Logf("%s (%s): non-compliant error payload: %s", scenario.Tool, scenario.Kind, payload)
		}
	}

	// memory_write duplicate title is operational but needs prior state.
	dupTitle := "NFR-DUP-TITLE-" + t.Name()
	requestID++
	dupResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name":      "memory_write",
			"arguments": map[string]any{"title": dupTitle, "content": "first"},
		},
		requestID,
	)
	dupPayload, dupIsError := toolCallResultPayload(t, dupResp)
	if dupIsError {
		t.Fatalf("memory_write seed for duplicate-title scenario failed: %s", dupPayload)
	}
	requestID++
	dupFailResp := handleTestMCPRequest(
		t,
		svc,
		mcp.MethodToolsCall,
		map[string]any{
			"name":      "memory_write",
			"arguments": map[string]any{"title": dupTitle, "content": "second"},
		},
		requestID,
	)
	dupFailPayload, dupFailIsError := toolCallResultPayload(t, dupFailResp)
	if !dupFailIsError {
		t.Fatalf("memory_write duplicate-title scenario expected error, got %s", dupFailPayload)
	}
	if isOperationalMCPErrorPayload(dupFailPayload) {
		compliant++
	} else {
		t.Logf("memory_write duplicate-title: non-compliant error payload: %s", dupFailPayload)
	}
	scenarios = append(scenarios, ToolNFRErrorScenario{
		Tool: "memory_write", Kind: NFRErrorScenarioOperational, Reason: "duplicate title domain failure",
	})

	ratio := float64(compliant) / float64(len(scenarios))
	if ratio < NFROperationalErrorCoverageMin {
		t.Fatalf(
			"operational error coverage=%.2f%% (%d/%d), want >= %.0f%%",
			ratio*100,
			compliant,
			len(scenarios),
			NFROperationalErrorCoverageMin*100,
		)
	}
}

func inventoryOperationalErrorScenarios(fixturePath string) []ToolNFRErrorScenario {
	scenarios := toolNFRErrorScenarios(fixturePath)
	filtered := make([]ToolNFRErrorScenario, 0, len(scenarios))
	for _, scenario := range scenarios {
		if scenario.Tool == "memory_write" && scenario.Kind == NFRErrorScenarioOperational {
			continue
		}
		filtered = append(filtered, scenario)
	}
	return filtered
}

func nfrErrorFixturePath(t *testing.T, svc *mcpServer) string {
	t.Helper()
	return requestContextFixturePath(t, svc)
}

func criticalToolOperationalErrorScenarios() []struct {
	tool string
	args map[string]any
} {
	fixturePath := "C:/workspace/Unit1.pas"
	scenarios := toolNFRErrorScenarios(fixturePath)
	legacy := make([]struct {
		tool string
		args map[string]any
	}, 0, len(scenarios))
	for _, scenario := range scenarios {
		legacy = append(legacy, struct {
			tool string
			args map[string]any
		}{tool: scenario.Tool, args: scenario.Args})
	}
	return legacy
}

func isOperationalMCPErrorPayload(payload string) bool {
	lower := strings.ToLower(payload)
	return strings.Contains(payload, "OP_") &&
		strings.Contains(lower, "action:") &&
		strings.Contains(lower, "recovery:")
}

func toolCallResultPayload(t *testing.T, response mcp.JSONRPCResponse) (string, bool) {
	t.Helper()

	resultBytes, err := json.Marshal(response.Result)
	if err != nil {
		t.Fatalf("marshal tool call result: %v", err)
	}

	var callResult map[string]any
	if err := json.Unmarshal(resultBytes, &callResult); err != nil {
		t.Fatalf("decode tool call result: %v", err)
	}

	isError, _ := callResult["isError"].(bool)
	return string(resultBytes), isError
}

func nfrPercentileDuration(samples []time.Duration, percentile int) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), samples...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	clamped := percentile
	if clamped < 0 {
		clamped = 0
	}
	if clamped > 100 {
		clamped = 100
	}

	rank := 1
	if clamped > 0 {
		rank = int(math.Ceil(float64(clamped) * float64(len(sorted)) / 100.0))
	}
	if rank < 1 {
		rank = 1
	}
	if rank > len(sorted) {
		rank = len(sorted)
	}
	return sorted[rank-1]
}
