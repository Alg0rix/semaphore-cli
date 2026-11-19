package cmd

import (
	"fmt"

	"semaphore-cli/internal/apiclient"
	"semaphore-cli/internal/output"

	"github.com/spf13/cobra"
)

func newInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show server information",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r := rt(cmd)
			var info apiclient.Info
			if err := r.api.Get("/info", &info); err != nil {
				return mapAPIError(err)
			}
			if r.printer.Format == "table" || r.printer.Format == "" {
				fmt.Printf("version:  %s\n", info.Version)
				if info.Ansible != "" {
					fmt.Printf("ansible:  %s\n", info.Ansible)
				}
				if info.WebHost != "" {
					fmt.Printf("web host: %s\n", info.WebHost)
				}
				return nil
			}
			return r.printer.Print(info, []output.Column{
				{Header: "VERSION", Value: func(v any) string { return v.(apiclient.Info).Version }},
			})
		},
	}
}
