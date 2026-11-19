package cmd

import (
	"fmt"
	"os"
	"sort"

	"semaphore-cli/internal/config"
	"semaphore-cli/internal/output"

	"github.com/spf13/cobra"
)

func newContextCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Manage saved servers (contexts)",
	}

	list := &cobra.Command{
		Use:         "list",
		Short:       "List saved contexts",
		Annotations: map[string]string{"skip-runtime": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := config.Path()
			if err != nil {
				return err
			}
			file, err := config.Load(path)
			if err != nil {
				return err
			}
			type row struct {
				Name    string `json:"name"`
				URL     string `json:"url"`
				Project string `json:"project"`
				Current bool   `json:"current"`
				Token   bool   `json:"token_saved"`
			}
			var names []string
			for n := range file.Contexts {
				names = append(names, n)
			}
			sort.Strings(names)
			rows := make([]row, 0, len(names))
			for _, n := range names {
				c := file.Contexts[n]
				rows = append(rows, row{Name: n, URL: c.URL, Project: c.Project, Current: n == file.Current, Token: c.Token != ""})
			}
			cols := []output.Column{
				{Header: "CURRENT", Value: func(v any) string {
					if v.(row).Current {
						return "*"
					}
					return ""
				}},
				{Header: "NAME", Value: func(v any) string { return v.(row).Name }},
				{Header: "URL", Value: func(v any) string { return v.(row).URL }},
				{Header: "PROJECT", Value: func(v any) string { return v.(row).Project }},
				{Header: "TOKEN", Value: func(v any) string {
					if v.(row).Token {
						return "saved"
					}
					return "-"
				}},
			}
			pr := output.New(os.Stdout, flagSet.Output, flagFields, flagQuiet, flagNoHeaders)
			return pr.Print(rows, cols)
		},
	}

	use := &cobra.Command{
		Use:         "use NAME",
		Short:       "Switch the current context",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"skip-runtime": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path, _ := config.Path()
			file, err := config.Load(path)
			if err != nil {
				return err
			}
			if _, ok := file.Contexts[args[0]]; !ok {
				return exit(exitNotFound, fmt.Errorf("context %q not found", args[0]))
			}
			file.Current = args[0]
			if err := file.Save(path); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Current context: %s\n", args[0])
			return nil
		},
	}

	set := &cobra.Command{
		Use:   "set NAME",
		Short: "Create or change a context",
		Long: `Create or change a context. Only the flags you pass are updated.

  semaphore-cli context set prod --url https://semaphore.example.com
  semaphore-cli context set prod -p 2
  semaphore-cli context set prod --insecure`,
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"skip-runtime": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path, _ := config.Path()
			file, err := config.Load(path)
			if err != nil {
				return err
			}
			c := file.Contexts[args[0]]
			if c == nil {
				c = &config.Context{}
				file.Contexts[args[0]] = c
			}
			ch := cmd.Flags().Changed
			if ch("url") {
				c.URL = flagSet.URL
			}
			if ch("token") {
				c.Token = flagSet.Token
			}
			if ch("project") {
				c.Project = flagSet.Project
			}
			if ch("insecure") {
				c.Insecure = flagSet.Insecure
			}
			if ch("ca-cert") {
				c.CACert = flagSet.CACert
			}
			if file.Current == "" {
				file.Current = args[0]
			}
			if err := file.Save(path); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Context %q saved.\n", args[0])
			return nil
		},
	}

	delete := &cobra.Command{
		Use:         "delete NAME",
		Short:       "Delete a context",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"skip-runtime": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path, _ := config.Path()
			file, err := config.Load(path)
			if err != nil {
				return err
			}
			if _, ok := file.Contexts[args[0]]; !ok {
				return exit(exitNotFound, fmt.Errorf("context %q not found", args[0]))
			}
			delete(file.Contexts, args[0])
			if file.Current == args[0] {
				file.Current = ""
				for n := range file.Contexts {
					file.Current = n
					break
				}
			}
			if err := file.Save(path); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Context %q deleted.\n", args[0])
			return nil
		},
	}

	cmd.AddCommand(list, use, set, delete)
	return cmd
}
