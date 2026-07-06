-- 给默认 AI 助手（agent_creator）装配两个新本地工具：
--   1) time.now   —— 返回当前时间（年/月/日 + UTC+8 + Unix 时间戳），纯本地无网络。
--   2) web.search —— 经 Tavily 联网搜索；运行时需 TAVILY_API_KEY，缺失则工具 fail-closed 不装配。
-- 运行时路径由 agentlogic.DefaultAssistantProvisioner.EnsureDefaultAssistant 幂等确保；
-- 本迁移面向存量库，与之等价（seed agent_tools + 绑定到 agent_creator 的 agent）。

do $$
begin
  if not exists (select 1 from accounts where identifier = 'agent_creator') then
    raise exception 'agent_creator must exist before binding time.now / web.search';
  end if;
end $$;

-- time.now
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
  'time.now',
  'Return the current time (year/month/day, UTC+8 clock, and Unix timestamp).',
  'local',
  'time.now',
  $${
    "type": "object",
    "additionalProperties": false,
    "properties": {}
  }$$::jsonb,
  $${
    "type": "object",
    "properties": {
      "year": {"type": "integer"},
      "month": {"type": "integer"},
      "day": {"type": "integer"},
      "timezone": {"type": "string"},
      "datetime": {"type": "string"},
      "date": {"type": "string"},
      "time": {"type": "string"},
      "weekday": {"type": "string"},
      "utc_datetime": {"type": "string"},
      "unix_seconds": {"type": "integer"},
      "unix_millis": {"type": "integer"},
      "iso8601": {"type": "string"}
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

-- web.search (Tavily)
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
  'web.search',
  'Search the web through Tavily and return ranked results with an optional answer summary.',
  'local',
  'web.search',
  $${
    "type": "object",
    "additionalProperties": false,
    "properties": {
      "query": {
        "type": "string",
        "description": "Search query keywords or question to look up on the web."
      },
      "max_results": {
        "type": "integer",
        "minimum": 1,
        "maximum": 10,
        "description": "Optional maximum number of results to return (default 5)."
      },
      "search_depth": {
        "type": "string",
        "enum": ["basic", "advanced"],
        "description": "Optional search depth. basic is faster (default); advanced digs deeper."
      }
    },
    "required": ["query"]
  }$$::jsonb,
  $${
    "type": "object",
    "properties": {
      "query": {"type": "string"},
      "answer": {"type": "string"},
      "results": {
        "type": "array",
        "items": {
          "type": "object",
          "properties": {
            "title": {"type": "string"},
            "url": {"type": "string"},
            "content": {"type": "string"},
            "score": {"type": "number"}
          }
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

-- 绑定两个工具到 agent_creator 的 agent。
insert into agent_tool_bindings (agent_id, tool_id, created_by)
select ag.agent_id, t.tool_id, a.account_id
from accounts a
join agents ag on ag.account_id = a.account_id
join agent_tools t on t.name in ('time.now', 'web.search')
where a.identifier = 'agent_creator'
on conflict (agent_id, tool_id) do update
set updated_at = now();
