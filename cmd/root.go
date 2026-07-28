package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/bandzoogle/datadog-cli/internal/dd"
	"github.com/bandzoogle/datadog-cli/internal/output"
	"github.com/spf13/cobra"
)

type globalOptions struct {
	site        string
	apiKey      string
	appKey      string
	accessToken string
	timeout     time.Duration
	debug       bool
	pretty      bool
	raw         bool
}

var globals globalOptions

var rootCmd = &cobra.Command{
	Use:     "ddcli",
	Version: Version,
	Short:   "Datadog CLI for scripts and LLM agents",
	Long: `ddcli wraps Datadog APIs with stable JSON output and narrow,
explicit write commands.

Credentials can be supplied with DD_API_KEY and DD_APP_KEY, or with
DD_ACCESS_TOKEN for OAuth-style bearer access tokens.`,
	SilenceUsage: true,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&globals.site, "site", "", "Datadog site, e.g. datadoghq.com, us3.datadoghq.com, datadoghq.eu")
	rootCmd.PersistentFlags().StringVar(&globals.apiKey, "api-key", "", "Datadog API key (prefer DD_API_KEY)")
	rootCmd.PersistentFlags().StringVar(&globals.appKey, "app-key", "", "Datadog application key (prefer DD_APP_KEY)")
	rootCmd.PersistentFlags().StringVar(&globals.accessToken, "access-token", "", "Datadog OAuth access token (prefer DD_ACCESS_TOKEN)")
	rootCmd.PersistentFlags().DurationVar(&globals.timeout, "timeout", 30*time.Second, "HTTP request timeout")
	rootCmd.PersistentFlags().BoolVar(&globals.debug, "debug", false, "Enable Datadog client debug logging")
	rootCmd.PersistentFlags().BoolVar(&globals.pretty, "pretty", false, "Pretty-print JSON output")
	rootCmd.PersistentFlags().BoolVar(&globals.raw, "raw", false, "Print raw Datadog response JSON without the ddcli envelope")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func datadogClient(cmd *cobra.Command) (*dd.Client, error) {
	return dd.NewClient(cmd.Context(), dd.Options{
		Site:        globals.site,
		APIKey:      globals.apiKey,
		AppKey:      globals.appKey,
		AccessToken: globals.accessToken,
		Debug:       globals.debug,
		Timeout:     globals.timeout,
	})
}

func outputOptions() output.Options {
	return output.Options{
		Pretty: globals.pretty,
		Raw:    globals.raw,
	}
}

func commandContext() context.Context {
	return context.Background()
}
