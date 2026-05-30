package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// OB-7: tracing 配置统一由 ConfigMap 注入的 env var 驱动（AGENTS_IM_TRACING_*，
// 见 deploy/k8s/configmap.yaml 与 TestProductionTracingConfigMapPointsAtOTelCollectorForTempo）。
// service yaml 不再保留 Tracing block，避免 14+ 份重复漂移；ResolveTracingConfig env 优先，
// go-zero config 的 Tracing 字段为 ,optional，缺省即由 env/默认值解析。
func TestServiceYAMLHasNoTracingBlock(t *testing.T) {
	for _, dir := range []string{"../../etc", "../../deploy/k8s/etc"} {
		matches, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
		if err != nil {
			t.Fatalf("glob %s: %v", dir, err)
		}
		if len(matches) == 0 {
			t.Fatalf("no yaml files found under %s", dir)
		}
		for _, configPath := range matches {
			content, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatalf("read %s: %v", configPath, err)
			}
			var doc map[string]any
			if err := yaml.Unmarshal(content, &doc); err != nil {
				t.Fatalf("parse %s: %v", configPath, err)
			}
			if _, ok := doc["Tracing"]; ok {
				t.Fatalf("%s still contains a Tracing block; tracing is configured via ConfigMap env (AGENTS_IM_TRACING_*), not per-service yaml", configPath)
			}
		}
	}
}

func TestProductionTracingConfigMapPointsAtOTelCollectorForTempo(t *testing.T) {
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
	if got := doc.Data["AGENTS_IM_OTLP_ENDPOINT"]; got != "otel-collector.agents-im.svc.cluster.local:4317" {
		t.Fatalf("AGENTS_IM_OTLP_ENDPOINT = %q", got)
	}
	if got := doc.Data["AGENTS_IM_OTLP_PROTOCOL"]; got != "grpc" {
		t.Fatalf("AGENTS_IM_OTLP_PROTOCOL = %q", got)
	}
	if got := doc.Data["AGENTS_IM_TRACE_UI_BASE_URL"]; got != "https://grafana.agenticim.xyz" {
		t.Fatalf("AGENTS_IM_TRACE_UI_BASE_URL = %q", got)
	}
	if _, ok := doc.Data["AGENTS_IM_JAEGER_BASE_URL"]; ok {
		t.Fatalf("AGENTS_IM_JAEGER_BASE_URL should not be present in production configmap")
	}
}
