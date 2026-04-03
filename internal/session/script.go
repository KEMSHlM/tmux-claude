package session

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ScriptConfig holds all parameters needed to generate a Claude Code launch script.
// Used by both local and SSH session types to produce a bash script.
type ScriptConfig struct {
	// SessionID is the unique session identifier (used for temp file naming).
	SessionID string

	// WorkDir is the directory to cd into before launching Claude.
	// Empty or "." means skip cd. For SSH, this is a remote path.
	WorkDir string

	// Flags are additional claude CLI flags (e.g. --resume).
	// Do not include --settings or --append-system-prompt here.
	Flags []string

	// MCP holds MCP server info for SSH lock file setup.
	// Nil for local sessions (hooks discover via existing lock files).
	MCP *MCPConfig

	// HooksJSON is the hooks settings JSON content to embed in the script.
	// Empty means skip hooks injection.
	HooksJSON string

	// SystemPrompt is injected via --append-system-prompt.
	// Empty means skip.
	SystemPrompt string

	// UserPrompt is passed as a positional argument to claude.
	// Empty means skip.
	UserPrompt string

	// SelfDelete causes the script to rm -f "$0" at startup.
	// Used for local temp scripts that should clean up after execution.
	SelfDelete bool
}

// MCPConfig holds MCP server connection info for SSH sessions.
type MCPConfig struct {
	Port  int
	Token string
}

// BuildScript generates bash script content for launching Claude Code.
// Handles both local and SSH contexts via ScriptConfig flags.
//
// The generated script follows a strict section order:
//  1. Shebang
//  2. Self-delete (if SelfDelete)
//  3. MCP lock file setup (if MCP != nil)
//  4. cd WorkDir
//  5. Hooks settings file (if HooksJSON non-empty)
//  6. System/user prompt variables (base64-encoded)
//  7. Auth environment variables
//  8. exec claude via login shell
func BuildScript(cfg ScriptConfig) (string, error) {
	var b strings.Builder
	b.WriteString("#!/bin/bash\n")

	// 1. Self-delete
	if cfg.SelfDelete {
		b.WriteString("rm -f \"$0\"\n")
	}

	// 2. MCP lock file setup
	if cfg.MCP != nil {
		if err := writeMCPSetup(&b, cfg.MCP); err != nil {
			return "", err
		}
	}

	// 3. cd WorkDir
	if cfg.WorkDir != "" && cfg.WorkDir != "." {
		b.WriteString(fmt.Sprintf("cd %s\n", posixQuote(cfg.WorkDir)))
	}

	// 4. Hooks settings file
	hooksPath := ""
	if cfg.HooksJSON != "" {
		p := "/tmp/lazyclaude/hooks-settings.json"
		b.WriteString("mkdir -p /tmp/lazyclaude\n")
		b.WriteString(fmt.Sprintf("cat > '%s' << 'HOOKSEOF'\n", p))
		b.WriteString(cfg.HooksJSON + "\n")
		b.WriteString("HOOKSEOF\n")
		hooksPath = p
	}

	// 5. System prompt and user prompt via base64 (avoids all quoting issues)
	sysPromptVar := ""
	if cfg.SystemPrompt != "" {
		encoded := base64.StdEncoding.EncodeToString([]byte(cfg.SystemPrompt))
		b.WriteString(fmt.Sprintf("_LC_SYSPROMPT=$(echo %s | base64 -d)\n", encoded))
		sysPromptVar = "_LC_SYSPROMPT"
	}

	userPromptVar := ""
	if strings.TrimSpace(cfg.UserPrompt) != "" {
		encoded := base64.StdEncoding.EncodeToString([]byte(cfg.UserPrompt))
		b.WriteString(fmt.Sprintf("_LC_USERPROMPT=$(echo %s | base64 -d)\n", encoded))
		userPromptVar = "_LC_USERPROMPT"
	}

	// 6. Auth environment variables
	writeAuthEnv(&b)

	// 7. Build the claude command and exec line
	claudeCmd := buildClaudeCmd(cfg.Flags, hooksPath, sysPromptVar, userPromptVar)
	b.WriteString(fmt.Sprintf("exec \"$SHELL\" -lic 'exec %s'\n", claudeCmd))

	return b.String(), nil
}

// writeMCPSetup writes the MCP lock file creation and cleanup trap.
func writeMCPSetup(b *strings.Builder, mcp *MCPConfig) error {
	lockDir := "$HOME/.claude/ide"
	lockFile := fmt.Sprintf("%s/%d.lock", lockDir, mcp.Port)

	lockContent := struct {
		PID       int    `json:"pid"`
		AuthToken string `json:"authToken"`
		Transport string `json:"transport"`
	}{PID: 0, AuthToken: mcp.Token, Transport: "ws"}
	lockJSON, err := json.Marshal(lockContent)
	if err != nil {
		return fmt.Errorf("marshal lock content: %w", err)
	}

	b.WriteString(fmt.Sprintf("mkdir -p \"%s\"\n", lockDir))
	b.WriteString(fmt.Sprintf("cat > \"%s\" << 'LOCKEOF'\n", lockFile))
	b.WriteString(string(lockJSON) + "\n")
	b.WriteString("LOCKEOF\n")
	b.WriteString(fmt.Sprintf("trap 'rm -f \"%s\"' EXIT\n", lockFile))
	return nil
}

// writeAuthEnv writes CLAUDE_CODE_AUTO_CONNECT_IDE and passthrough auth tokens.
func writeAuthEnv(b *strings.Builder) {
	b.WriteString("export CLAUDE_CODE_AUTO_CONNECT_IDE=true\n")
	for _, key := range []string{"CLAUDE_CODE_OAUTH_TOKEN", "ANTHROPIC_API_KEY", "CLAUDE_CODE_API_KEY"} {
		if val := os.Getenv(key); val != "" {
			b.WriteString(fmt.Sprintf("export %s=%s\n", key, posixQuote(val)))
		}
	}
}

// buildClaudeCmd constructs the claude command string for the -lic argument.
//
// The exec line uses single quotes: exec "$SHELL" -lic 'exec claude ...'.
// The outer bash passes the single-quoted content as a literal string to the
// login shell. The login shell then interprets that string as a command,
// expanding "$_LC_SYSPROMPT" etc. from exported environment variables.
// This is safe because:
//  1. Prompt values are base64-decoded into exported variables before exec
//  2. The -lic argument only contains variable NAMES (not values)
//  3. Double quotes around "$var" inside the login shell command prevent
//     word splitting of the expanded value
func buildClaudeCmd(flags []string, hooksPath, sysPromptVar, userPromptVar string) string {
	var parts []string
	parts = append(parts, "claude")

	if hooksPath != "" {
		parts = append(parts, "--settings", hooksPath)
	}

	for _, f := range flags {
		parts = append(parts, f)
	}

	if sysPromptVar != "" {
		parts = append(parts, "--append-system-prompt", fmt.Sprintf(`"$%s"`, sysPromptVar))
	}

	if userPromptVar != "" {
		parts = append(parts, fmt.Sprintf(`"$%s"`, userPromptVar))
	}

	return strings.Join(parts, " ")
}
