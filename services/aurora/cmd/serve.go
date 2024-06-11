package cmd

import (
	"github.com/spf13/cobra"
	aurora "github.com/HashCash-Consultants/go/services/aurora/internal"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "run aurora server",
	Long:  "serve initializes then starts the aurora HTTP server",
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := aurora.NewAppFromFlags(globalConfig, globalFlags)
		if err != nil {
			return err
		}
		return app.Serve()
	},
}

func init() {
	RootCmd.AddCommand(serveCmd)
}
