package main

import (
	"bufio"
	"fmt"
	"net"
	"os"

	"github.com/spf13/cobra"
)

func newAskpassCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "askpass [prompt]",
		Short:  "SSH_ASKPASS helper (internal use)",
		Hidden: true,
		Args:   cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sockPath := os.Getenv("LAZYCLAUDE_ASKPASS_SOCK")
			if sockPath == "" {
				return fmt.Errorf("LAZYCLAUDE_ASKPASS_SOCK not set")
			}

			prompt := "Password: "
			if len(args) > 0 {
				prompt = args[0]
			}

			conn, err := net.Dial("unix", sockPath)
			if err != nil {
				return fmt.Errorf("connect to askpass socket: %w", err)
			}
			defer conn.Close()

			// Send prompt (line-based protocol).
			if _, err := fmt.Fprintln(conn, prompt); err != nil {
				return fmt.Errorf("send prompt: %w", err)
			}

			// Read response.
			scanner := bufio.NewScanner(conn)
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					return fmt.Errorf("read response: %w", err)
				}
				return fmt.Errorf("no response from askpass server")
			}

			response := scanner.Text()
			if response == "" {
				// Empty response means user cancelled.
				os.Exit(1)
			}

			// Output to stdout without newline — SSH reads this.
			fmt.Fprint(os.Stdout, response)
			return nil
		},
	}
}
