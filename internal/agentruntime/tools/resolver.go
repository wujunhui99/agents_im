package tools

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/internal/model"
)

const defaultPermissionLevel = "agent_bound"

var forbiddenIdentifierFragments = []string{
	"shell",
	"stdio",
	"local_process",
	"process.spawn",
	"exec",
	"command",
	"script",
	"filesystem.write",
	"fs.write",
	"file.write",
	"write_file",
	"delete_file",
	"remove_file",
	"py" + "thon",
}

var forbiddenConfigKeys = map[string]struct{}{
	"args":        {},
	"argv":        {},
	"binary":      {},
	"cmd":         {},
	"command":     {},
	"cwd":         {},
	"entrypoint":  {},
	"env":         {},
	"environment": {},
	"executable":  {},
	"interpreter": {},
	"process":     {},
	"program":     {},
	"script":      {},
	"shell":       {},
	"stdio":       {},
}

type ResolverOption func(*Resolver)

func WithAdapterCatalog(catalog AdapterCatalog) ResolverOption {
	return func(r *Resolver) {
		r.catalog = catalog
	}
}

func WithResolutionAuditHook(hook ResolutionAuditHook) ResolverOption {
	return func(r *Resolver) {
		r.auditHook = hook
	}
}

type Resolver struct {
	registry  Registry
	catalog   AdapterCatalog
	auditHook ResolutionAuditHook
}

func NewResolver(registry Registry, opts ...ResolverOption) (*Resolver, error) {
	if registry == nil {
		return nil, apperror.InvalidArgument("agent tool registry is required")
	}
	r := &Resolver{registry: registry}
	for _, opt := range opts {
		if opt != nil {
			opt(r)
		}
	}
	return r, nil
}

func (r *Resolver) ResolveAgentTools(ctx context.Context, req ResolveAgentToolsRequest) ([]ResolvedTool, error) {
	if ctx == nil {
		return nil, apperror.InvalidArgument("context is required")
	}
	agentID, err := normalizeRequiredID(req.AgentID, "agent_id")
	if err != nil {
		return nil, err
	}

	explicitToolIDs := len(req.ToolIDs) > 0
	toolIDs, err := normalizeToolIDs(req.ToolIDs)
	if err != nil {
		return nil, err
	}
	if !explicitToolIDs {
		bindings, err := r.registry.ListToolBindings(ctx, agentID)
		if err != nil {
			return nil, err
		}
		toolIDs = make([]string, 0, len(bindings))
		for _, binding := range bindings {
			toolID := strings.TrimSpace(binding.ToolID)
			if toolID != "" {
				toolIDs = append(toolIDs, toolID)
			}
		}
	}

	resolved := make([]ResolvedTool, 0, len(toolIDs))
	for _, toolID := range toolIDs {
		tool, err := r.resolveBoundTool(ctx, ResolveToolRequest{
			AgentID:         agentID,
			ToolID:          toolID,
			RequireAdapters: req.RequireAdapters,
			RunID:           req.RunID,
			TraceID:         req.TraceID,
			RequestID:       req.RequestID,
		}, !explicitToolIDs)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, tool)
	}
	return resolved, nil
}

func (r *Resolver) ResolveTool(ctx context.Context, req ResolveToolRequest) (ResolvedTool, error) {
	if ctx == nil {
		return ResolvedTool{}, apperror.InvalidArgument("context is required")
	}
	agentID, err := normalizeRequiredID(req.AgentID, "agent_id")
	if err != nil {
		return ResolvedTool{}, err
	}
	toolID, err := normalizeRequiredID(req.ToolID, "tool_id")
	if err != nil {
		return ResolvedTool{}, err
	}
	req.AgentID = agentID
	req.ToolID = toolID
	return r.resolveBoundTool(ctx, req, false)
}

func (r *Resolver) resolveBoundTool(ctx context.Context, req ResolveToolRequest, bindingAlreadyLoaded bool) (ResolvedTool, error) {
	if !bindingAlreadyLoaded {
		if _, err := r.registry.GetToolBinding(ctx, req.AgentID, req.ToolID); err != nil {
			if isNotFound(err) {
				return ResolvedTool{}, r.deny(ctx, req, "", apperror.Forbidden("tool is not bound to agent"))
			}
			return ResolvedTool{}, err
		}
	}

	tool, err := r.registry.GetTool(ctx, req.ToolID)
	if err != nil {
		return ResolvedTool{}, err
	}
	spec, err := r.safeSpec(ctx, tool)
	if err != nil {
		return ResolvedTool{}, r.deny(ctx, req, tool.Name, err)
	}

	var adapter ToolAdapter
	if r.catalog != nil {
		foundAdapter, found, err := r.catalog.LookupToolAdapter(spec)
		if err != nil {
			return ResolvedTool{}, r.deny(ctx, req, spec.Name, err)
		}
		if found {
			if foundAdapter == nil {
				return ResolvedTool{}, r.deny(ctx, req, spec.Name, apperror.Internal("tool adapter catalog returned a nil adapter"))
			}
			adapterSpec := foundAdapter.Spec()
			if strings.TrimSpace(adapterSpec.ToolID) != spec.ToolID {
				return ResolvedTool{}, r.deny(ctx, req, spec.Name, apperror.Internal("tool adapter catalog returned an adapter for a different tool"))
			}
			adapter = foundAdapter
		}
	}
	if req.RequireAdapters && adapter == nil {
		return ResolvedTool{}, r.deny(ctx, req, spec.Name, apperror.Forbidden("safe tool adapter is not configured"))
	}

	resolved := ResolvedTool{Spec: spec, Adapter: adapter}
	if err := r.record(ctx, ResolutionAuditEvent{
		RunID:     strings.TrimSpace(req.RunID),
		AgentID:   req.AgentID,
		ToolID:    spec.ToolID,
		ToolName:  spec.Name,
		Status:    ResolutionStatusAllowed,
		TraceID:   strings.TrimSpace(req.TraceID),
		RequestID: strings.TrimSpace(req.RequestID),
	}); err != nil {
		return ResolvedTool{}, err
	}
	return resolved, nil
}

func (r *Resolver) safeSpec(ctx context.Context, tool model.AgentTool) (ToolSpec, error) {
	tool.ToolID = strings.TrimSpace(tool.ToolID)
	tool.Name = strings.TrimSpace(tool.Name)
	tool.Description = strings.TrimSpace(tool.Description)
	tool.MCPServerID = strings.TrimSpace(tool.MCPServerID)
	tool.MCPToolName = strings.TrimSpace(tool.MCPToolName)
	tool.LocalHandlerKey = strings.TrimSpace(tool.LocalHandlerKey)
	tool.BuiltinKey = strings.TrimSpace(tool.BuiltinKey)
	tool.PermissionLevel = normalizePermissionLevel(tool.PermissionLevel)

	if tool.ToolID == "" {
		return ToolSpec{}, apperror.InvalidArgument("tool_id is required")
	}
	if tool.Name == "" {
		return ToolSpec{}, apperror.InvalidArgument("tool name is required")
	}
	if tool.Status != model.AgentToolStatusActive {
		return ToolSpec{}, apperror.Forbidden("tool is not active")
	}
	if !tool.AdminConfigured {
		return ToolSpec{}, apperror.Forbidden("tool must be admin configured")
	}
	if reason := unsafeToolIdentifier(tool.Name); reason != "" && !allowedUnsafeLocalToolName(tool) {
		return ToolSpec{}, apperror.Forbidden("tool name is not allowed: " + reason)
	}
	inputSchema, err := normalizeJSON(tool.InputSchemaJSON, "input_schema_json")
	if err != nil {
		return ToolSpec{}, err
	}
	outputSchema, err := normalizeJSON(tool.OutputSchemaJSON, "output_schema_json")
	if err != nil {
		return ToolSpec{}, err
	}

	spec := ToolSpec{
		ToolID:           tool.ToolID,
		Name:             tool.Name,
		Description:      tool.Description,
		ToolType:         tool.ToolType,
		InputSchemaJSON:  inputSchema,
		OutputSchemaJSON: outputSchema,
		PermissionLevel:  tool.PermissionLevel,
	}
	switch tool.ToolType {
	case model.AgentToolTypeMCP:
		mcp, err := r.safeMCPToolSpec(ctx, tool)
		if err != nil {
			return ToolSpec{}, err
		}
		spec.MCP = &mcp
	case model.AgentToolTypeLocal:
		if tool.MCPServerID != "" || tool.MCPToolName != "" || tool.BuiltinKey != "" {
			return ToolSpec{}, apperror.Forbidden("local tool metadata is not allowed")
		}
		if !allowedLocalHandlerKey(tool.LocalHandlerKey) {
			return ToolSpec{}, apperror.Forbidden("local tool handler is not whitelisted")
		}
		spec.Local = &LocalToolSpec{HandlerKey: tool.LocalHandlerKey}
	case model.AgentToolTypeBuiltin:
		if tool.MCPServerID != "" || tool.MCPToolName != "" || tool.LocalHandlerKey != "" {
			return ToolSpec{}, apperror.Forbidden("builtin tool metadata is not allowed")
		}
		if !allowedBuiltinKey(tool.BuiltinKey) {
			return ToolSpec{}, apperror.Forbidden("builtin tool key is not whitelisted")
		}
		spec.Builtin = &BuiltinToolSpec{BuiltinKey: tool.BuiltinKey}
	default:
		return ToolSpec{}, apperror.Forbidden("tool type is not allowed")
	}
	return spec, nil
}

func (r *Resolver) safeMCPToolSpec(ctx context.Context, tool model.AgentTool) (MCPToolSpec, error) {
	if tool.MCPServerID == "" {
		return MCPToolSpec{}, apperror.Forbidden("mcp tool server is required")
	}
	if tool.MCPToolName == "" {
		return MCPToolSpec{}, apperror.Forbidden("mcp tool name is required")
	}
	if tool.LocalHandlerKey != "" || tool.BuiltinKey != "" {
		return MCPToolSpec{}, apperror.Forbidden("mcp tool metadata is not allowed")
	}
	if reason := unsafeToolIdentifier(tool.MCPToolName); reason != "" {
		return MCPToolSpec{}, apperror.Forbidden("mcp tool name is not allowed: " + reason)
	}

	server, err := r.registry.GetMCPServer(ctx, tool.MCPServerID)
	if err != nil {
		return MCPToolSpec{}, err
	}
	server.ServerID = strings.TrimSpace(server.ServerID)
	server.Name = strings.TrimSpace(server.Name)
	server.URL = strings.TrimSpace(server.URL)
	server.HeadersSecretRef = strings.TrimSpace(server.HeadersSecretRef)

	if server.Status != model.AgentToolStatusActive {
		return MCPToolSpec{}, apperror.Forbidden("mcp server is not active")
	}
	if !server.AdminConfigured {
		return MCPToolSpec{}, apperror.Forbidden("mcp server must be admin configured")
	}
	if !safeMCPTransport(server.Transport) {
		return MCPToolSpec{}, apperror.Forbidden("mcp server transport is not allowed")
	}
	if err := validateHTTPURL(server.URL, "mcp server url"); err != nil {
		return MCPToolSpec{}, err
	}
	if server.TimeoutSeconds <= 0 {
		return MCPToolSpec{}, apperror.InvalidArgument("mcp server timeout_seconds must be greater than 0")
	}
	configJSON, err := normalizeJSON(server.ConfigJSON, "mcp server config_json")
	if err != nil {
		return MCPToolSpec{}, err
	}
	if forbiddenPath, err := findForbiddenConfigMetadata(configJSON); err != nil {
		return MCPToolSpec{}, err
	} else if forbiddenPath != "" {
		return MCPToolSpec{}, apperror.Forbidden("mcp server config contains local process metadata: " + forbiddenPath)
	}

	return MCPToolSpec{
		ServerID:         server.ServerID,
		ServerName:       server.Name,
		Transport:        server.Transport,
		URL:              server.URL,
		ConfigJSON:       configJSON,
		HeadersSecretRef: server.HeadersSecretRef,
		TimeoutSeconds:   server.TimeoutSeconds,
		ToolName:         tool.MCPToolName,
	}, nil
}

func (r *Resolver) deny(ctx context.Context, req ResolveToolRequest, toolName string, err error) error {
	auditErr := r.record(ctx, ResolutionAuditEvent{
		RunID:     strings.TrimSpace(req.RunID),
		AgentID:   strings.TrimSpace(req.AgentID),
		ToolID:    strings.TrimSpace(req.ToolID),
		ToolName:  strings.TrimSpace(toolName),
		Status:    ResolutionStatusDenied,
		Reason:    err.Error(),
		TraceID:   strings.TrimSpace(req.TraceID),
		RequestID: strings.TrimSpace(req.RequestID),
	})
	if auditErr != nil {
		return errors.Join(err, auditErr)
	}
	return err
}

func (r *Resolver) record(ctx context.Context, event ResolutionAuditEvent) error {
	if r.auditHook == nil {
		return nil
	}
	return r.auditHook.RecordToolResolution(ctx, event)
}

func normalizeRequiredID(value string, field string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", apperror.InvalidArgument(field + " is required")
	}
	return value, nil
}

func normalizeToolIDs(values []string) ([]string, error) {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		toolID := strings.TrimSpace(value)
		if toolID == "" {
			return nil, apperror.InvalidArgument("tool_id is required")
		}
		if _, ok := seen[toolID]; ok {
			continue
		}
		seen[toolID] = struct{}{}
		normalized = append(normalized, toolID)
	}
	return normalized, nil
}

func normalizePermissionLevel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultPermissionLevel
	}
	return value
}

func normalizeJSON(raw string, field string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "{}", nil
	}
	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return "", apperror.InvalidArgument(field + " must be valid JSON")
	}
	encoded, err := json.Marshal(decoded)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func findForbiddenConfigMetadata(normalizedJSON string) (string, error) {
	var decoded any
	if err := json.Unmarshal([]byte(normalizedJSON), &decoded); err != nil {
		return "", apperror.InvalidArgument("mcp server config_json must be valid JSON")
	}
	return findForbiddenConfigPath(decoded, ""), nil
}

func findForbiddenConfigPath(value any, path string) string {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			normalizedKey := strings.ToLower(strings.TrimSpace(key))
			childPath := joinConfigPath(path, key)
			if _, forbidden := forbiddenConfigKeys[normalizedKey]; forbidden {
				return childPath
			}
			if normalizedKey == "transport" {
				if transport, ok := child.(string); ok && strings.EqualFold(strings.TrimSpace(transport), "stdio") {
					return childPath
				}
			}
			if nested := findForbiddenConfigPath(child, childPath); nested != "" {
				return nested
			}
		}
	case []any:
		for _, child := range typed {
			if nested := findForbiddenConfigPath(child, path); nested != "" {
				return nested
			}
		}
	}
	return ""
}

func joinConfigPath(parent string, key string) string {
	key = strings.TrimSpace(key)
	if parent == "" {
		return key
	}
	if key == "" {
		return parent
	}
	return parent + "." + key
}

func validateHTTPURL(raw string, field string) error {
	if raw == "" {
		return apperror.InvalidArgument(field + " is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return apperror.InvalidArgument(field + " must be a valid http or https URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return apperror.InvalidArgument(field + " must be a valid http or https URL")
	}
	return nil
}

func safeMCPTransport(transport model.AgentMCPTransport) bool {
	switch transport {
	case model.AgentMCPTransportHTTP, model.AgentMCPTransportSSE, model.AgentMCPTransportStreamableHTTP:
		return true
	default:
		return false
	}
}

func allowedLocalHandlerKey(key string) bool {
	switch strings.TrimSpace(key) {
	case model.LocalToolHandlerGetConversationContext,
		model.LocalToolHandlerReadSkillFile,
		model.LocalToolHandlerSendAgentMessage,
		model.LocalToolHandlerPythonExecute,
		model.LocalToolHandlerAgentCreate:
		return true
	default:
		return false
	}
}

func allowedUnsafeLocalToolName(tool model.AgentTool) bool {
	return tool.ToolType == model.AgentToolTypeLocal &&
		tool.Name == model.LocalToolHandlerPythonExecute &&
		tool.LocalHandlerKey == model.LocalToolHandlerPythonExecute
}

func allowedBuiltinKey(key string) bool {
	switch strings.TrimSpace(key) {
	case model.BuiltinToolReadConversationContext,
		model.BuiltinToolReadSkillFile,
		model.BuiltinToolSendAgentMessage:
		return true
	default:
		return false
	}
}

func unsafeToolIdentifier(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return ""
	}
	for _, fragment := range forbiddenIdentifierFragments {
		if strings.Contains(normalized, fragment) {
			return fragment
		}
	}
	return ""
}

func isNotFound(err error) bool {
	appErr := apperror.From(err)
	return appErr != nil && appErr.Code == apperror.CodeNotFound
}
