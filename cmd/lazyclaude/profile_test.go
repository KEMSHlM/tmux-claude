package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeProfileConfig writes a config.json to $HOME/.lazyclaude/ under the given
// temporary home directory and returns the config path.
func writeProfileConfig(t *testing.T, home, content string) string {
	t.Helper()
	dir := filepath.Join(home, ".lazyclaude")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	path := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

// runProfileList executes `profile list` with the supplied extra args and
// returns stdout, stderr, and the command error.
func runProfileList(t *testing.T, extraArgs ...string) (stdout, stderr string, err error) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	cmd := newProfileListCmd()
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(extraArgs)
	err = cmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

// TestProfileList_NoConfig verifies that when no config.json exists the
// builtin default profile is listed with a '*' in the DEFAULT column.
func TestProfileList_NoConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	out, _, err := runProfileList(t)
	require.NoError(t, err)

	assert.Contains(t, out, "NAME")
	assert.Contains(t, out, "DEFAULT")
	assert.Contains(t, out, "COMMAND")
	assert.Contains(t, out, "DESCRIPTION")
	assert.Contains(t, out, "default")
	assert.Contains(t, out, "claude")
	assert.Contains(t, out, "(builtin)")
	// The builtin default should be marked
	lines := strings.Split(out, "\n")
	var defaultLine string
	for _, l := range lines {
		if strings.Contains(l, "default") && !strings.Contains(l, "DEFAULT") {
			defaultLine = l
			break
		}
	}
	assert.Contains(t, defaultLine, "*")
}

// TestProfileList_DefaultOutput verifies the 4-column table format with a
// user-defined default profile.
func TestProfileList_DefaultOutput(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	writeProfileConfig(t, home, `{
		"version": 1,
		"profiles": [
			{"name": "opus", "command": "claude", "args": ["--model=opus-4"], "description": "Opus 1M"},
			{"name": "sonnet", "command": "claude", "description": "Sonnet 4.6", "default": true}
		]
	}`)

	out, _, err := runProfileList(t)
	require.NoError(t, err)

	// ARGS and ENV must NOT appear in default (non-verbose) mode.
	assert.NotContains(t, out, "ARGS")
	assert.NotContains(t, out, "ENV")

	// sonnet has default:true so it should carry '*'.
	lines := strings.Split(out, "\n")
	var sonnetLine, opusLine string
	for _, l := range lines {
		switch {
		case strings.Contains(l, "sonnet"):
			sonnetLine = l
		case strings.Contains(l, "opus") && !strings.Contains(l, "COMMAND"):
			opusLine = l
		}
	}
	assert.Contains(t, sonnetLine, "*", "sonnet (default:true) should carry '*'")
	// opus should not have the default mark
	fields := strings.Fields(opusLine)
	// fields[0]=opus fields[1]=<default-col> ...
	if len(fields) >= 2 {
		assert.NotEqual(t, "*", fields[1], "opus default column should be empty")
	}
}

// TestProfileList_Verbose verifies that -v adds ARGS and ENV columns.
func TestProfileList_Verbose(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	writeProfileConfig(t, home, `{
		"version": 1,
		"profiles": [
			{
				"name": "opus",
				"command": "claude",
				"args": ["--model=opus-4"],
				"env": {"ANTHROPIC_MODEL": "opus"},
				"description": "Opus 1M"
			}
		]
	}`)

	out, _, err := runProfileList(t, "-v")
	require.NoError(t, err)

	assert.Contains(t, out, "ARGS")
	assert.Contains(t, out, "ENV")
	assert.Contains(t, out, "--model=opus-4")
	assert.Contains(t, out, "ANTHROPIC_MODEL=opus")
}

// TestProfileList_JSON verifies --json emits a JSON array that includes the
// builtin flag for the built-in default profile.
func TestProfileList_JSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	writeProfileConfig(t, home, `{
		"version": 1,
		"profiles": [
			{"name": "opus", "command": "claude", "args": ["--model=opus-4"], "description": "Opus 1M"}
		]
	}`)

	out, _, err := runProfileList(t, "--json")
	require.NoError(t, err)

	var entries []profileListEntry
	require.NoError(t, json.Unmarshal([]byte(out), &entries))
	require.Len(t, entries, 2, "opus + builtin default")

	var opus, builtin *profileListEntry
	for i := range entries {
		switch entries[i].Name {
		case "opus":
			opus = &entries[i]
		case "default":
			builtin = &entries[i]
		}
	}

	require.NotNil(t, opus)
	assert.Equal(t, "claude", opus.Command)
	assert.Equal(t, []string{"--model=opus-4"}, opus.Args)
	assert.Equal(t, "Opus 1M", opus.Description)
	assert.False(t, opus.Builtin)
	assert.False(t, opus.EffectiveDefault, "opus has no default:true so should not be effective default")

	require.NotNil(t, builtin)
	assert.True(t, builtin.Builtin, "builtin default should have builtin=true")
	assert.True(t, builtin.EffectiveDefault, "builtin 'default' profile is the effective default when no user default is set")
}

// TestProfileList_JSON_EffectiveDefault verifies that effective_default tracks
// the resolved default, not merely the default:true flag.
func TestProfileList_JSON_EffectiveDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// "sonnet" has default:true, so it is the effective default.
	// "opus" has no default:true but is listed first.
	writeProfileConfig(t, home, `{
		"version": 1,
		"profiles": [
			{"name": "opus",   "command": "claude"},
			{"name": "sonnet", "command": "claude", "default": true}
		]
	}`)

	out, _, err := runProfileList(t, "--json")
	require.NoError(t, err)

	var entries []profileListEntry
	require.NoError(t, json.Unmarshal([]byte(out), &entries))

	byName := make(map[string]*profileListEntry)
	for i := range entries {
		byName[entries[i].Name] = &entries[i]
	}

	require.NotNil(t, byName["sonnet"])
	assert.True(t, byName["sonnet"].EffectiveDefault, "sonnet (default:true) should be effective default")

	require.NotNil(t, byName["opus"])
	assert.False(t, byName["opus"].EffectiveDefault, "opus should not be effective default")

	// builtin 'default' profile must not steal the effective_default mark.
	if d, ok := byName["default"]; ok {
		assert.False(t, d.EffectiveDefault, "builtin default should not be effective default when sonnet has default:true")
	}
}

// TestProfileList_BrokenConfig verifies that a malformed config.json returns
// an error containing "invalid JSON" with line/col information.
func TestProfileList_BrokenConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	writeProfileConfig(t, home, `{invalid json`)

	_, _, err := runProfileList(t)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON")
}

// TestProfileList_MultipleDefaults verifies that when multiple profiles have
// default:true, a warning is written to stderr and the first default is used.
func TestProfileList_MultipleDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	writeProfileConfig(t, home, `{
		"version": 1,
		"profiles": [
			{"name": "alpha", "command": "claude", "default": true},
			{"name": "beta",  "command": "claude", "default": true}
		]
	}`)

	out, stderr, err := runProfileList(t)
	require.NoError(t, err)
	assert.Contains(t, stderr, "warning")

	// The first default (alpha) should carry '*'.
	lines := strings.Split(out, "\n")
	var alphaLine string
	for _, l := range lines {
		if strings.Contains(l, "alpha") {
			alphaLine = l
			break
		}
	}
	assert.Contains(t, alphaLine, "*")
}

// TestProfileList_ConfigFooter verifies that the config path footer is present
// in table output but absent in JSON output.
func TestProfileList_ConfigFooter(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// No config: only builtin
	outTable, _, err := runProfileList(t)
	require.NoError(t, err)
	assert.Contains(t, outTable, "Config:")

	outJSON, _, err := runProfileList(t, "--json")
	require.NoError(t, err)
	assert.NotContains(t, outJSON, "Config:")
}
