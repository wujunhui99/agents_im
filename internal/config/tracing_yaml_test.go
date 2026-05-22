package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestTracingYAMLCoverageForAllBackendServices(t *testing.T) {
	expected := []string{
		"agent-api", "auth-api", "friends-api", "gateway-ws", "groups-api", "message-api", "message-transfer", "user-api",
		"auth-rpc", "friends-rpc", "groups-rpc", "mail-rpc", "message-rpc", "user-rpc",
	}
	for _, dir := range []struct {
		path          string
		wantEnabled   bool
		wantEndpoint  string
		localSafeMode bool
	}{
		{path: "../../deploy/k8s/etc", wantEnabled: true, wantEndpoint: "${AGENTS_IM_OTLP_ENDPOINT}"},
		{path: "../../etc", wantEnabled: false, localSafeMode: true},
	} {
		for _, serviceName := range expected {
			configPath := filepath.Join(dir.path, serviceName+".yaml")
			content, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatalf("read %s: %v", configPath, err)
			}
			var doc map[string]any
			if err := yaml.Unmarshal(content, &doc); err != nil {
				t.Fatalf("parse %s: %v", configPath, err)
			}
			tracing, ok := doc["Tracing"].(map[string]any)
			if !ok {
				t.Fatalf("%s missing Tracing block", configPath)
			}
			if got, _ := tracing["ServiceName"].(string); got != serviceName {
				t.Fatalf("%s Tracing.ServiceName = %q, want %q", configPath, got, serviceName)
			}
			if got, _ := tracing["Enabled"].(bool); got != dir.wantEnabled {
				t.Fatalf("%s Tracing.Enabled = %v, want %v", configPath, got, dir.wantEnabled)
			}
			if dir.wantEndpoint != "" {
				if got, _ := tracing["OTLPEndpoint"].(string); got != dir.wantEndpoint {
					t.Fatalf("%s Tracing.OTLPEndpoint = %q, want %q", configPath, got, dir.wantEndpoint)
				}
			}
			if dir.localSafeMode {
				if got, _ := tracing["OTLPEndpoint"].(string); got != "" {
					t.Fatalf("%s local Tracing.OTLPEndpoint should be empty unless explicitly enabled, got %q", configPath, got)
				}
			}
		}
	}
}

func TestProductionTracingConfigMapPointsAtJaegerCollector(t *testing.T) {
	content, err := os.ReadFile("../../deploy/k8s/configmap.yaml")
	if err != nil {
		t.Fatalf("read configmap: %v", err)
	}
	var doc struct {
		Data map[string]string `yaml:"data"`
	}
	if err := yaml.Unmarshal(content, &doc); err != nil {
		t.Fatalf("parse configmap: %v", err)
	}
	if got := doc.Data["AGENTS_IM_TRACING_ENABLED"]; got != "true" {
		t.Fatalf("AGENTS_IM_TRACING_ENABLED = %q", got)
	}
	if got := doc.Data["AGENTS_IM_OTLP_ENDPOINT"]; got != "jaeger-collector.agents-im.svc.cluster.local:4317" {
		t.Fatalf("AGENTS_IM_OTLP_ENDPOINT = %q", got)
	}
	if got := doc.Data["AGENTS_IM_OTLP_PROTOCOL"]; got != "grpc" {
		t.Fatalf("AGENTS_IM_OTLP_PROTOCOL = %q", got)
	}
}
