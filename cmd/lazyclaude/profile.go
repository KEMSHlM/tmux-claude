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
// EffectiveDefault is set to true for the profile that would be selected when
// an empty profile name is requested, matching the '*' mark in table output.
// Note: Default reflects the user-supplied config flag; EffectiveDefault is
// the resolved result of profile.ResolveDefault.
type profileListEntry struct {
	Name             string            `json:"name"`
	Command          string            `json:"command"`
	Args             []string          `json:"args,omitempty"`
	Env              map[string]string `json:"env,omitempty"`
	Description      string            `json:"description,omitempty"`
	Default          bool              `json:"default,omitempty"`
	Builtin          bool              `json:"builtin,omitempty"`
	EffectiveDefault bool              `json:"effective_default,omitempty"`
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

			// Resolve the effective default once; warnings come as a second
			// return value from the same call.
			defaultProf, warnings := profile.ResolveDefault(profiles)
			for _, w := range warnings {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning:", w)
			}

			if asJSON {
				return renderProfilesJSON(cmd, profiles, defaultProf)
			}

			if err := renderProfilesTable(cmd, profiles, defaultProf, verbose); err != nil {
				return fmt.Errorf("render table: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\nConfig: %s\n", configPath)
			fmt.Fprintln(cmd.OutOrStdout(), "Use `-v` for full details, or `--json` for JSON output.")
			return nil
		},
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "show full profile details including args and env")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output profiles as JSON")
	return cmd
}

func renderProfilesJSON(cmd *cobra.Command, profiles []profile.ProfileDef, defaultProf profile.ProfileDef) error {
	entries := make([]profileListEntry, len(profiles))
	for i, p := range profiles {
		entries[i] = profileListEntry{
			Name:             p.Name,
			Command:          p.Command,
			Args:             p.Args,
			Env:              p.Env,
			Description:      p.Description,
			Default:          p.Default,
			Builtin:          p.Builtin,
			EffectiveDefault: p.Name == defaultProf.Name,
		}
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetEscapeHTML(false)
	return enc.Encode(entries)
}

func renderProfilesTable(cmd *cobra.Command, profiles []profile.ProfileDef, defaultProf profile.ProfileDef, verbose bool) error {
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
	return tw.Flush()
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
