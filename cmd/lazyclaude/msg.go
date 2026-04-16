package main

import (
	"fmt"
	"strings"

	"github.com/any-context/lazyclaude/internal/core/config"
	"github.com/any-context/lazyclaude/internal/server"
	"github.com/spf13/cobra"
)

// validMsgTypes mirrors the server's allowlist.
var validMsgTypes = map[string]bool{
	"review_request":  true,
	"review_response": true,
	"status":          true,
	"done":            true,
	"issue":           true,
}

func newMsgCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "msg",
		Short: "Manage inter-session messaging",
	}

	cmd.AddCommand(newMsgSendCmd())
	cmd.AddCommand(newMsgCreateCmd())
	return cmd
}

func newMsgSendCmd() *cobra.Command {
	var (
		msgType string
		from    string
	)

	cmd := &cobra.Command{
		Use:   "send <session-id> <message>",
		Short: "Send a message to a session by ID",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetID := args[0]
			body := strings.Join(args[1:], " ")

			if !validMsgTypes[msgType] {
				return fmt.Errorf("invalid --type %q; must be one of: review_request, review_response, status, done, issue", msgType)
			}

			paths := config.DefaultPaths()
			disc, err := server.DiscoverServer(paths.IDEDir)
			if err != nil {
				return fmt.Errorf("discover server: %w", err)
			}

			client := server.NewClient(disc.Port, disc.Token)

			if err := client.SendMessage(cmd.Context(), from, targetID, msgType, body); err != nil {
				return fmt.Errorf("send message: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Message sent to %s\n", targetID)
			return nil
		},
	}

	cmd.Flags().StringVar(&msgType, "type", "status", "message type (review_request, review_response, status, done, issue)")
	cmd.Flags().StringVar(&from, "from", "cli", "sender session ID")

	return cmd
}

func newMsgCreateCmd() *cobra.Command {
	var (
		name        string
		createType  string
		prompt      string
		from        string
		profileName string
		options     string
		parentID    string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new session via the server API",
		RunE: func(cmd *cobra.Command, args []string) error {
			switch createType {
			case "worker", "local":
				// valid
			default:
				return fmt.Errorf("invalid --type %q; must be one of: worker, local", createType)
			}

			paths := config.DefaultPaths()
			disc, err := server.DiscoverServer(paths.IDEDir)
			if err != nil {
				return fmt.Errorf("discover server: %w", err)
			}

			client := server.NewClient(disc.Port, disc.Token)

			// Sugar: when --parent is not specified and --from is a PM session,
			// automatically use --from as parentID.
			if parentID == "" && from != "" && from != "cli" {
				parentID = resolveParentFromSender(cmd, client, from)
			}

			result, err := client.CreateSession(cmd.Context(), from, name, createType, prompt, profileName, options, parentID)
			if err != nil {
				return fmt.Errorf("create session: %w", err)
			}

			if result.Session != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Created session %s (id=%s, role=%s, path=%s)\n",
					result.Session.Name, result.Session.ID, result.Session.Role, result.Session.Path)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Session created (status=%s)\n", result.Status)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "session name (required)")
	_ = cmd.MarkFlagRequired("name")
	cmd.Flags().StringVar(&createType, "type", "worker", "session type (worker, local)")
	cmd.Flags().StringVar(&prompt, "prompt", "", "initial prompt for the session")
	cmd.Flags().StringVar(&from, "from", "cli", "caller session ID")
	cmd.Flags().StringVar(&profileName, "profile", "", "launch profile name (empty uses effective default)")
	cmd.Flags().StringVar(&options, "options", "", "extra flags passed to the claude invocation (space-separated)")
	cmd.Flags().StringVar(&parentID, "parent", "", "parent PM session ID (auto-detected from --from if omitted)")

	return cmd
}

// resolveParentFromSender checks whether the --from session has role "pm"
// and returns its ID as the implicit parent. Returns "" if the session is
// not a PM or if the lookup fails (best-effort).
func resolveParentFromSender(cmd *cobra.Command, client *server.Client, from string) string {
	sessions, err := client.Sessions(cmd.Context())
	if err != nil {
		return ""
	}
	for _, s := range sessions {
		if s.ID == from && s.Role == "pm" {
			return from
		}
	}
	return ""
}
