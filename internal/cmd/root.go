package cmd

import (
	"github.com/spf13/cobra"
)

var (
	sessionName string
	silent      bool
	rateLim     int
	jitterMs    string
	proxy       string
	outputPath  string
	format      string
)

func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "god-hunter",
		Short: "God-Hunter: Professional Bug Bounty Framework",
		Long: `God-Hunter is a modular, stateful exploitation framework for professional
security researchers operating within authorized bug bounty program scopes.

It targets vulnerability classes missed by public scanners:
- Business Logic flaws
- IAM Trust Relationship abuse
- Mobile Deep Link exploitation

Usage:
  god-hunter <module>:<action> [target] [flags]`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return nil
		},
	}

	rootCmd.PersistentFlags().StringVarP(&sessionName, "session", "s", "", "Attach to or create named session")
	rootCmd.PersistentFlags().BoolVarP(&silent, "silent", "q", false, "Suppress all non-critical output")
	rootCmd.PersistentFlags().IntVarP(&rateLim, "rate", "r", 30, "Requests per minute cap")
	rootCmd.PersistentFlags().StringVarP(&jitterMs, "jitter", "j", "800-2400", "Jitter range in ms")
	rootCmd.PersistentFlags().StringVarP(&proxy, "proxy", "p", "", "Upstream proxy (Burp/Caido/MITM)")
	rootCmd.PersistentFlags().StringVarP(&outputPath, "out", "o", "./findings", "Output path for findings")
	rootCmd.PersistentFlags().StringVarP(&format, "format", "f", "json", "Output format: json | markdown | burp-xml")

	// Register module commands
	apiCmd := NewAPICommand()
	rootCmd.AddCommand(apiCmd)

	return rootCmd
}
