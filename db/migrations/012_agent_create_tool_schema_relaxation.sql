-- Issue #166: carry forward agent.create schema changes without modifying applied migration 011.
-- Migration 011 was already applied in production; keep it immutable and update the tool schema here.

update agent_tools
set input_schema_json = $${
    "type": "object",
    "additionalProperties": false,
    "properties": {
      "identifier": {
        "type": "string",
        "description": "Optional unique account identifier. If omitted the server allocates one."
      },
      "name": {
        "type": "string",
        "description": "Display name for the new Agent account and Agent profile."
      },
      "description": {
        "type": "string",
        "description": "Human-facing Agent purpose or job description."
      },
      "system_prompt": {
        "type": "string",
        "description": "Optional system prompt to bind as the Agent definition. If omitted, the service generates one from name and description."
      },
      "tool_names": {
        "type": "array",
        "items": {"type": "string"},
        "description": "Optional low-risk tool names to bind. High-risk write, Python, MCP/network, and agent.create tools are rejected by policy."
      }
    },
    "required": ["name", "description"]
  }$$::jsonb,
    updated_at = now()
where name = 'agent.create';
