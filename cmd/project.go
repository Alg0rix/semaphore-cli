package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/Alg0rix/semaphore-cli/internal/apiclient"
	"github.com/Alg0rix/semaphore-cli/internal/output"

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

	var bf bodyFlags
	var name, alertChat, ptype string
	var alert, demo bool
	var maxParallel int

	bind := func(c *cobra.Command) {
		c.Flags().StringVar(&name, "name", "", "project name")
		c.Flags().BoolVar(&alert, "alert", false, "send alerts to the chat")
		c.Flags().StringVar(&alertChat, "alert-chat", "", "alert chat id")
		c.Flags().IntVar(&maxParallel, "max-parallel", 0, "max parallel tasks (0 = unlimited)")
		c.Flags().StringVar(&ptype, "type", "", "project type")
		bf.bind(c)
	}

	body := func(cmd *cobra.Command, r *runtime) (map[string]any, error) {
		m, err := bf.read(r.in)
		if err != nil {
			return nil, err
		}
		flagInto(cmd, m, "name", "name")
		flagInto(cmd, m, "alert", "alert")
		flagInto(cmd, m, "alert-chat", "alert_chat")
		flagInto(cmd, m, "max-parallel", "max_parallel_tasks")
		flagInto(cmd, m, "type", "type")
		return m, nil
	}

	create := &cobra.Command{
		Use:   "create",
		Short: "Create a project",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r := rt(cmd)
			m, err := body(cmd, r)
			if err != nil {
				return exit(exitUsage, err)
			}
			flagInto(cmd, m, "demo", "demo")
			var p apiclient.Project
			if err := r.api.Post("/projects", m, &p); err != nil {
				return mapAPIError(err)
			}
			return r.printer.Print(p, projectColumns())
		},
	}
	create.Flags().BoolVar(&demo, "demo", false, "create demo project resources")
	bind(create)

	update := &cobra.Command{
		Use:   "update <id|name>",
		Short: "Update a project: passed flags and --data fields replace current values",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r := rt(cmd)
			cur, err := findItem(cmd, "/projects", "name", args[0])
			if err != nil {
				return err
			}
			over, err := body(cmd, r)
			if err != nil {
				return exit(exitUsage, err)
			}
			merge(cur, over)
			pid, _ := cur["id"].(float64)
			if err := r.api.Put(fmt.Sprintf("/project/%d/", int(pid)), cur, nil); err != nil {
				return mapAPIError(err)
			}
			var p apiclient.Project
			raw, _ := json.Marshal(cur)
			_ = json.Unmarshal(raw, &p)
			return r.printer.Print(p, projectColumns())
		},
	}
	bind(update)

	del := &cobra.Command{
		Use:   "delete <id|name>",
		Short: "Delete a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pid, err := resolveName(cmd, "/projects", "name", args[0])
			if err != nil {
				return err
			}
			if err := confirm(cmd, fmt.Sprintf("Delete project %q (id %d) with all its tasks?", args[0], pid)); err != nil {
				return err
			}
			if err := rt(cmd).api.Delete(fmt.Sprintf("/project/%d/", pid)); err != nil {
				return mapAPIError(err)
			}
			fmt.Fprintf(stderr, "project %d deleted\n", pid)
			return nil
		},
	}

	cmd.AddCommand(list, get, create, update, del)
	return cmd
}
