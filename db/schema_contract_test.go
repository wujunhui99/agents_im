package db

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestInternalAgentIDMigrationConvertsDatabaseGeneratedIDsToBigint(t *testing.T) {
	migration := readMigration(t, "013_internal_agent_ids_bigint.sql")

	requiredBigintColumns := []string{
		"agents.agent_id",
		"agent_prompts.prompt_id",
		"mcp_servers.server_id",
		"agent_tools.tool_id",
		"agent_tools.mcp_server_id",
		"agent_skills.skill_id",
		"agent_prompt_bindings.agent_id",
		"agent_prompt_bindings.prompt_id",
		"agent_tool_bindings.agent_id",
		"agent_tool_bindings.tool_id",
		"agent_skill_bindings.agent_id",
		"agent_skill_bindings.skill_id",
		"agent_runs.run_id",
		"agent_runs.agent_id",
		"agent_tool_calls.tool_call_id",
		"agent_tool_calls.run_id",
		"agent_tool_calls.agent_id",
		"agent_tool_calls.tool_id",
		"agent_file_reads.file_read_id",
		"agent_file_reads.run_id",
		"agent_file_reads.agent_id",
		"agent_file_reads.skill_id",
		"agent_python_execs.python_exec_id",
		"agent_python_execs.run_id",
		"agent_python_execs.agent_id",
	}
	for _, tableColumn := range requiredBigintColumns {
		table, column, ok := strings.Cut(tableColumn, ".")
		if !ok {
			t.Fatalf("invalid table.column fixture: %s", tableColumn)
		}
		if !altersColumnToBigint(migration, table, column) {
			t.Fatalf("migration 013 must convert %s to bigint", tableColumn)
		}
	}

	for _, tableColumn := range []string{
		"agents.agent_id",
		"agent_prompts.prompt_id",
		"mcp_servers.server_id",
		"agent_tools.tool_id",
		"agent_skills.skill_id",
		"agent_runs.run_id",
		"agent_tool_calls.tool_call_id",
		"agent_file_reads.file_read_id",
		"agent_python_execs.python_exec_id",
	} {
		table, column, _ := strings.Cut(tableColumn, ".")
		if !addsIdentity(migration, table, column) {
			t.Fatalf("migration 013 must add identity generation to %s", tableColumn)
		}
	}
}

func TestInternalAgentIDMigrationDoesNotAddNewPrefixedTextDefaults(t *testing.T) {
	migration := readMigration(t, "013_internal_agent_ids_bigint.sql")
	prefixedIDDefault := regexp.MustCompile(`(?i)_id\s+text\s+primary\s+key\s+default\s+\('[a-z_]+_'\s*\|\|`)
	if match := prefixedIDDefault.FindString(migration); match != "" {
		t.Fatalf("migration 013 creates prefixed text id default %q; use bigint identity instead", match)
	}
}

func readMigration(t *testing.T, name string) string {
	t.Helper()
	content, err := os.ReadFile("migrations/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func altersColumnToBigint(migration, table, column string) bool {
	pattern := regexp.MustCompile(`(?is)alter\s+table\s+if\s+exists\s+` + regexp.QuoteMeta(table) + `\s+alter\s+column\s+` + regexp.QuoteMeta(column) + `\s+type\s+bigint\b`)
	return pattern.MatchString(migration)
}

func addsIdentity(migration, table, column string) bool {
	pattern := regexp.MustCompile(`(?is)alter\s+table\s+if\s+exists\s+` + regexp.QuoteMeta(table) + `\s+alter\s+column\s+` + regexp.QuoteMeta(column) + `\s+add\s+generated\s+by\s+default\s+as\s+identity\b`)
	return pattern.MatchString(migration)
}
