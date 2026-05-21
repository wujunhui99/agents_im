-- Source of truth for Issue #129 default assistant agent.create tool backfill.
-- Apply with db/migrations/011_default_assistant_agent_create_tool.sql in normal migration order.

do $$
begin
  if not exists (select 1 from accounts where identifier = 'agent_creator') then
    raise exception 'agent_creator must exist before binding agent.create';
  end if;
end $$;

update agent_prompts
set content = '你是一个通用 AI 助手，回答应准确、简洁、友好。你可以帮助用户解释概念、比较方案、整理信息、生成文本和提供编程/产品建议。不要编造事实；不确定时说明不确定并给出可验证的下一步。当用户明确要求创建新的 Agent 时，可以使用 agent.create 工具创建账号、Agent 配置、系统提示词、允许的低风险工具绑定，并把新 Agent 加为该用户好友。',
    updated_at = now()
where name = 'agent_creator_default_system_prompt'
  and version = 'v1';

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
  'agent.create',
  'Create a new Agent through the server-side agent assembly workflow.',
  'local',
  'agent.create',
  $${
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
  $${
    "type": "object",
    "properties": {
      "agent_id": {"type": "string"},
      "account_id": {"type": "string"},
      "identifier": {"type": "string"},
      "name": {"type": "string"},
      "description": {"type": "string"},
      "prompt_id": {"type": "string"},
      "tool_names": {
        "type": "array",
        "items": {"type": "string"}
      },
      "friend_user_id": {"type": "string"}
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
join agent_tools t on t.name = 'agent.create'
where a.identifier = 'agent_creator'
on conflict (agent_id, tool_id) do update
set updated_at = now();
