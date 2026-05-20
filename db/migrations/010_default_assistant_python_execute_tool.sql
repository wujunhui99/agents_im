-- Issue #108: make the built-in agent_creator assistant able to use python.execute.
-- Execution remains fail-closed unless PythonExecutor.Backend is explicitly configured.

do $$
begin
  if not exists (select 1 from accounts where identifier = 'agent_creator') then
    raise exception 'agent_creator must exist before binding python.execute';
  end if;
end $$;

insert into agent_tools (
  name,
  description,
  tool_type,
  local_handler_key,
  input_schema_json,
  output_schema_json,
  permission_level,
  status,
  admin_configured,
  created_by
)
select
  'python.execute',
  'Execute bounded Python code through the configured sandbox executor.',
  'local',
  'python.execute',
  $${
    "type": "object",
    "additionalProperties": false,
    "properties": {
      "code": {
        "type": "string",
        "description": "Python code to execute in the configured sandbox."
      },
      "timeout_seconds": {
        "type": "integer",
        "minimum": 1,
        "maximum": 30,
        "description": "Optional execution timeout in seconds."
      },
      "files": {
        "type": "array",
        "items": {"type": "string"},
        "description": "Optional read-only allowlisted file paths. Empty unless explicitly configured."
      }
    },
    "required": ["code"]
  }$$::jsonb,
  $${
    "type": "object",
    "properties": {
      "stdout": {"type": "string"},
      "stderr": {"type": "string"},
      "result_json": {},
      "exit_code": {"type": "integer"},
      "timed_out": {"type": "boolean"},
      "output_truncated": {"type": "boolean"},
      "error": {
        "type": ["object", "null"],
        "properties": {
          "code": {"type": "string"},
          "message": {"type": "string"}
        }
      }
    }
  }$$::jsonb,
  'restricted',
  'active',
  true,
  account_id
from accounts
where identifier = 'agent_creator'
on conflict (name) do update
set description = excluded.description,
    tool_type = excluded.tool_type,
    mcp_server_id = null,
    mcp_tool_name = '',
    local_handler_key = excluded.local_handler_key,
    builtin_key = '',
    input_schema_json = excluded.input_schema_json,
    output_schema_json = excluded.output_schema_json,
    permission_level = excluded.permission_level,
    status = excluded.status,
    admin_configured = excluded.admin_configured,
    created_by = excluded.created_by,
    updated_at = now();

insert into agent_tool_bindings (agent_id, tool_id, created_by)
select ag.agent_id, t.tool_id, a.account_id
from accounts a
join agents ag on ag.account_id = a.account_id
join agent_tools t on t.name = 'python.execute'
where a.identifier = 'agent_creator'
on conflict (agent_id, tool_id) do update
set updated_at = now();
