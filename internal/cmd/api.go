package cmd

import (
	"github.com/god-hunter/god-hunter/internal/api"
	"github.com/spf13/cobra"
)

func NewAPICommand() *cobra.Command {
	apiCmd := &cobra.Command{
		Use:   "api",
		Short: "API chain recording, replay, fuzzing, and race condition detection",
		Long: `Manage API call sequences (chains), replay them with mutations,
and detect race conditions and business logic vulnerabilities.

Examples:
  god-hunter api record https://api.example.com
  god-hunter api replay --session sess_001 --mutate race
  god-hunter api fuzz https://api.example.com --wordlist payloads.txt
  god-hunter api chain https://api.example.com --record
  god-hunter api race https://api.example.com --concurrent 20`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// :record action
	recordCmd := &cobra.Command{
		Use:   "record <url>",
		Short: "Intercept and store API call sequences",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return api.RecordChain(cmd.Context(), args, sessionName, proxy, silent)
		},
	}
	apiCmd.AddCommand(recordCmd)

	// :replay action
	replayCmd := &cobra.Command{
		Use:   "replay",
		Short: "Replay a Chain with optional mutation",
		RunE: func(cmd *cobra.Command, args []string) error {
			return api.ReplayChain(cmd.Context(), sessionName, silent)
		},
	}
	replayCmd.Flags().String("mutate", "", "Mutation strategy: race | order | param | timing")
	apiCmd.AddCommand(replayCmd)

	// :fuzz action
	fuzzCmd := &cobra.Command{
		Use:   "fuzz <url>",
		Short: "Parameter fuzzing with contextual wordlists",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return api.FuzzParameters(cmd.Context(), args, sessionName, silent)
		},
	}
	fuzzCmd.Flags().String("wordlist", "", "Custom wordlist path")
	fuzzCmd.Flags().String("param", "", "Specific parameter to fuzz")
	apiCmd.AddCommand(fuzzCmd)

	// :chain action
	chainCmd := &cobra.Command{
		Use:   "chain <url>",
		Short: "Multi-step API sequence builder",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return api.BuildChain(cmd.Context(), args, sessionName, silent)
		},
	}
	chainCmd.Flags().Bool("record", false, "Record the chain automatically")
	apiCmd.AddCommand(chainCmd)

	// :race action
	raceCmd := &cobra.Command{
		Use:   "race <url>",
		Short: "Concurrent request engine for race conditions",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return api.RaceDetection(cmd.Context(), args, sessionName, silent)
		},
	}
	raceCmd.Flags().Int("concurrent", 10, "Number of concurrent requests")
	raceCmd.Flags().Int("iterations", 100, "Number of test iterations")
	apiCmd.AddCommand(raceCmd)

	// :mutate action
	mutateCmd := &cobra.Command{
		Use:   "mutate",
		Short: "Advanced mutation testing for vulnerability detection",
		Long: `Perform sophisticated mutation testing on stored API chains
to detect vulnerabilities via order variations, timing deviations,
parameter mutations, and state inconsistencies.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return api.TestMutations(cmd.Context(), args, sessionName, silent)
		},
	}
	mutateCmd.Flags().String("strategy", "all", "Mutation strategy: order|timing|param|state|race|all")
	mutateCmd.Flags().Int("iterations", 1, "Number of mutation iterations")
	apiCmd.AddCommand(mutateCmd)

	return apiCmd
}
