package cmd

import (
	"fmt"

	"semaphore-cli/internal/apiclient"
	"semaphore-cli/internal/output"

	"github.com/spf13/cobra"
)

func projectColumns() []output.Column {
	return []output.Column{
		col[apiclient.Project]("ID", func(p apiclient.Project) int { return p.ID }),
		col[apiclient.Project]("NAME", func(p apiclient.Project) string { return p.Name }),
		col[apiclient.Project]("CREATED", func(p apiclient.Project) any { return p.Created.Format("2006-01-02") }),
		col[apiclient.Project]("ALERT", func(p apiclient.Project) bool { return p.Alert }),
	}
}

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "project", Short: "Projects"}

	list := &cobra.Command{
		Use:   "list",
		Short: "List projects",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r := rt(cmd)
			var projects []apiclient.Project
			if err := r.api.Get("/projects", &projects); err != nil {
				return mapAPIError(err)
			}
			return r.printer.Print(projects, projectColumns())
		},
	}

	get := &cobra.Command{
		Use:   "get [id|name]",
		Short: "Show one project (the configured one by default)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r := rt(cmd)
			arg := r.cfg.Project
			if len(args) > 0 {
				arg = args[0]
			}
			var projects []apiclient.Project
			if err := r.api.Get("/projects", &projects); err != nil {
				return mapAPIError(err)
			}
			for _, p := range projects {
				if fmt.Sprint(p.ID) == arg || p.Name == arg {
					return r.printer.Print(p, projectColumns())
				}
			}
			return exit(exitNotFound, fmt.Errorf("project %q not found", arg))
		},
	}

	cmd.AddCommand(list, get)
	return cmd
}
