package presentation

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ToolDisplay holds parsed tool information for rendering.
type ToolDisplay struct {
	Name    string
	CWD     string
	Lines   []string // formatted body lines
}

// ParseToolInput parses tool name and JSON input into display lines.
func ParseToolInput(toolName, toolInput, cwd string) ToolDisplay {
	td := ToolDisplay{
		Name: toolName,
		CWD:  cwd,
	}

	var input map[string]any
	if err := json.Unmarshal([]byte(toolInput), &input); err != nil {
		td.Lines = []string{toolInput}
		return td
	}

	switch toolName {
	case "Bash":
		td.Lines = formatBash(input)
	case "Read":
		td.Lines = formatRead(input)
	case "Write", "Edit":
		td.Lines = formatWriteEdit(toolName, input)
	case "Agent", "Task":
		td.Lines = formatAgent(input)
	default:
		td.Lines = formatGeneric(input)
	}

	return td
}

// FormatToolLines renders the full tool popup body with styled output.
func FormatToolLines(td ToolDisplay) []string {
	var lines []string
	lines = append(lines, "")

	switch td.Name {
	case "Bash":
		lines = append(lines, Dim+"  Command:"+Reset)
		for _, l := range td.Lines {
			lines = append(lines, FgCyan+"  $ "+Reset+l)
		}
	case "Edit":
		for _, l := range td.Lines {
			if strings.HasPrefix(l, "  - ") {
				lines = append(lines, FgRed+l+Reset)
			} else if strings.HasPrefix(l, "  + ") {
				lines = append(lines, FgGreen+l+Reset)
			} else {
				lines = append(lines, "  "+l)
			}
		}
	default:
		for _, l := range td.Lines {
			lines = append(lines, "  "+l)
		}
	}

	if td.CWD != "" {
		lines = append(lines, "")
		lines = append(lines, Dim+"  CWD: "+Reset+FgDimGray+td.CWD+Reset)
	}

	lines = append(lines, "")
	return lines
}

func formatBash(input map[string]any) []string {
	cmd, _ := input["command"].(string)
	if cmd == "" {
		return []string{"(empty command)"}
	}
	return strings.Split(cmd, "\n")
}

func formatRead(input map[string]any) []string {
	path, _ := input["file_path"].(string)
	if path == "" {
		return []string{"(no file path)"}
	}
	return []string{fmt.Sprintf("File: %s", path)}
}

func formatWriteEdit(name string, input map[string]any) []string {
	path, _ := input["file_path"].(string)
	var lines []string
	if path != "" {
		lines = append(lines, fmt.Sprintf("File: %s", path))
	}

	if name == "Edit" {
		old, _ := input["old_string"].(string)
		new_, _ := input["new_string"].(string)
		if old != "" {
			lines = append(lines, "")
			lines = append(lines, "Old:")
			for _, l := range strings.Split(old, "\n") {
				lines = append(lines, "  - "+l)
			}
		}
		if new_ != "" {
			lines = append(lines, "New:")
			for _, l := range strings.Split(new_, "\n") {
				lines = append(lines, "  + "+l)
			}
		}
	}

	return lines
}

func formatAgent(input map[string]any) []string {
	desc, _ := input["description"].(string)
	if desc == "" {
		desc = "(no description)"
	}
	return []string{desc}
}

func formatGeneric(input map[string]any) []string {
	data, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return []string{"(unable to format)"}
	}
	return strings.Split(string(data), "\n")
}
