package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/any-context/lazyclaude/internal/profile"
	"github.com/spf13/cobra"
)

func newProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage launch profiles",
	}
	cmd.AddCommand(newProfileListCmd())
	return cmd
}

// profileListEntry is the JSON shape emitted by --json.
// ProfileDef.Builtin carries json:"-" so we use a local struct to expose it.
type profileListEntry struct {
	Name        string            `json:"name"`
	Command     string            `json:"command"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Description string            `json:"description,omitempty"`
	Default     bool              `json:"default,omitempty"`
	Builtin     bool              `json:"builtin,omitempty"`
}

func newProfileListCmd() *cobra.Command {
	var (
		verbose bool
		asJSON  bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available launch profiles",
		RunE: func(cmd *cobra.Command, _ []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("determine home directory: %w", err)
			}
			configPath := filepath.Join(home, ".lazyclaude", "config.json")

			_, profiles, err := profile.Load(configPath)
			if err != nil {
				return fmt.Errorf("%s: %w", configPath, err)
			}

			// Emit warnings about multiple default markers before any output.
			_, warnings := profile.ResolveDefault(profiles)
			for _, w := range warnings {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning:", w)
			}

			if asJSON {
				return renderProfilesJSON(cmd, profiles)
			}

			defaultProf, _ := profile.ResolveDefault(profiles)
			renderProfilesTable(cmd, profiles, defaultProf, verbose)

			fmt.Fprintf(cmd.OutOrStdout(), "\nConfig: %s\n", configPath)
			fmt.Fprintln(cmd.OutOrStdout(), "Use `-v` for full details, or `--json` for JSON output.")
			return nil
		},
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "show full profile details including args and env")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output profiles as JSON")
	return cmd
}

func renderProfilesJSON(cmd *cobra.Command, profiles []profile.ProfileDef) error {
	entries := make([]profileListEntry, len(profiles))
	for i, p := range profiles {
		entries[i] = profileListEntry{
			Name:        p.Name,
			Command:     p.Command,
			Args:        p.Args,
			Env:         p.Env,
			Description: p.Description,
			Default:     p.Default,
			Builtin:     p.Builtin,
		}
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetEscapeHTML(false)
	return enc.Encode(entries)
}

func renderProfilesTable(cmd *cobra.Command, profiles []profile.ProfileDef, defaultProf profile.ProfileDef, verbose bool) {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	if verbose {
		fmt.Fprintln(tw, "NAME\tDEFAULT\tCOMMAND\tDESCRIPTION\tARGS\tENV")
	} else {
		fmt.Fprintln(tw, "NAME\tDEFAULT\tCOMMAND\tDESCRIPTION")
	}

	for _, p := range profiles {
		defMark := ""
		if p.Name == defaultProf.Name {
			defMark = "*"
		}
		desc := p.Description
		if desc == "" && p.Builtin {
			desc = "(builtin)"
		}
		if verbose {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
				p.Name, defMark, p.Command, desc,
				strings.Join(p.Args, " "),
				envString(p.Env),
			)
		} else {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", p.Name, defMark, p.Command, desc)
		}
	}
	tw.Flush()
}

// envString converts an env map to a deterministic KEY=VALUE string.
func envString(env map[string]string) string {
	if len(env) == 0 {
		return ""
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + env[k]
	}
	return strings.Join(parts, " ")
}
