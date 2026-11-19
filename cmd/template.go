package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/Alg0rix/semaphore-cli/internal/apiclient"
	"github.com/Alg0rix/semaphore-cli/internal/output"

	"github.com/spf13/cobra"
)

func templateColumns() []output.Column {
	return []output.Column{
		col[apiclient.Template]("ID", func(t apiclient.Template) int { return t.ID }),
		col[apiclient.Template]("NAME", func(t apiclient.Template) string { return t.Name }),
		col[apiclient.Template]("APP", func(t apiclient.Template) string { return t.App }),
		col[apiclient.Template]("PLAYBOOK", func(t apiclient.Template) string { return t.Playbook }),
		col[apiclient.Template]("LAST STATUS", func(t apiclient.Template) string {
			if t.LastTask == nil {
				return "-"
			}
			return t.LastTask.Status
		}),
	}
}

func listTemplates(cmd *cobra.Command, pid int) ([]apiclient.Template, error) {
	var tpls []apiclient.Template
	if err := rt(cmd).api.Get(fmt.Sprintf("/project/%d/templates", pid), &tpls); err != nil {
		return nil, err
	}
	return tpls, nil
}

func resolveTemplate(cmd *cobra.Command, pid int, arg string) (*apiclient.Template, error) {
	tpls, err := listTemplates(cmd, pid)
	if err != nil {
		return nil, mapAPIError(err)
	}
	if n, err := strconv.Atoi(arg); err == nil {
		for _, t := range tpls {
			if t.ID == n {
				return &t, nil
			}
		}
		return nil, exit(exitNotFound, fmt.Errorf("template %d not found", n))
	}
	for _, t := range tpls {
		if strings.EqualFold(t.Name, arg) {
			return &t, nil
		}
	}
	return nil, exit(exitNotFound, fmt.Errorf("template %q not found", arg))
}

func newTemplateCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "template", Short: "Templates"}

	list := &cobra.Command{
		Use:   "list",
		Short: "List templates of the project",
		RunE: func(cmd *cobra.Command, _ []string) error {
			pid, err := projectID(cmd)
			if err != nil {
				return err
			}
			tpls, err := listTemplates(cmd, pid)
			if err != nil {
				return mapAPIError(err)
			}
			return rt(cmd).printer.Print(tpls, templateColumns())
		},
	}

	get := &cobra.Command{
		Use:   "get <id|name>",
		Short: "Show one template",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pid, err := projectID(cmd)
			if err != nil {
				return err
			}
			tpl, err := resolveTemplate(cmd, pid, args[0])
			if err != nil {
				return mapAPIError(err)
			}
			return rt(cmd).printer.Print(tpl, templateColumns())
		},
	}

	var rf runFlags
	run := &cobra.Command{
		Use:   "run <id|name>",
		Short: "Start a task from a template",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pid, err := projectID(cmd)
			if err != nil {
				return err
			}
			tpl, err := resolveTemplate(cmd, pid, args[0])
			if err != nil {
				return err
			}
			return runTask(cmd, pid, apiclient.TaskRequest{TemplateID: tpl.ID}, &rf)
		},
	}
	rf.bind(run)

	cmd.AddCommand(list, get, run)
	cmd.AddCommand(templateCrudCmds()...)
	return cmd
}

// templateCrudCmds adds create/update/delete. Scalar fields come from
// flags; anything else from --data/--file JSON. inventory, repository,
// environment, view and build-template accept an id or a name.
func templateCrudCmds() []*cobra.Command {
	var bf bodyFlags
	var name, app, playbook, desc, ttype, args, limit, branch, workdir string
	var inv, repo, env, view, build string
	var allowArgs, autorun, suppOK, suppErr bool

	bind := func(c *cobra.Command) {
		c.Flags().StringVar(&name, "name", "", "template name")
		c.Flags().StringVar(&app, "app", "", "app: ansible, terraform, bash, python, pulumi, powershell")
		c.Flags().StringVar(&playbook, "playbook", "", "playbook/script path in the repository")
		c.Flags().StringVar(&desc, "description", "", "description")
		c.Flags().StringVar(&ttype, "type", "", "template type: empty, build or deploy")
		c.Flags().StringVar(&args, "args", "", "extra CLI arguments (JSON array string)")
		c.Flags().StringVar(&limit, "limit", "", "default host limit")
		c.Flags().StringVar(&branch, "git-branch", "", "override git branch")
		c.Flags().StringVar(&workdir, "working-directory", "", "repository-relative working directory")
		c.Flags().StringVar(&inv, "inventory", "", "inventory id or name")
		c.Flags().StringVar(&repo, "repository", "", "repository id or name")
		c.Flags().StringVar(&env, "environment", "", "environment id or name")
		c.Flags().StringVar(&view, "view", "", "view id or name")
		c.Flags().StringVar(&build, "build-template", "", "build template id or name (deploy type)")
		c.Flags().BoolVar(&allowArgs, "allow-override-args", false, "tasks may override arguments")
		c.Flags().BoolVar(&autorun, "autorun", false, "run after build (deploy type)")
		c.Flags().BoolVar(&suppOK, "suppress-success-alerts", false, "no alerts on success")
		c.Flags().BoolVar(&suppErr, "suppress-error-alerts", false, "no alerts on failure")
		bf.bind(c)
	}

	body := func(cmd *cobra.Command, pid int) (map[string]any, error) {
		m, err := bf.read(rt(cmd).in)
		if err != nil {
			return nil, err
		}
		for _, f := range [][2]string{
			{"name", "name"}, {"app", "app"}, {"playbook", "playbook"},
			{"description", "description"}, {"type", "type"}, {"args", "arguments"},
			{"limit", "limit"}, {"git-branch", "git_branch"},
			{"working-directory", "working_directory"},
			{"allow-override-args", "allow_override_args_in_task"},
			{"autorun", "autorun"}, {"suppress-success-alerts", "suppress_success_alerts"},
			{"suppress-error-alerts", "suppress_error_alerts"},
		} {
			flagInto(cmd, m, f[0], f[1])
		}
		for _, ref := range []struct{ flag, key, path, nameKey string }{
			{"inventory", "inventory_id", "/inventory", "name"},
			{"repository", "repository_id", "/repositories", "name"},
			{"environment", "environment_id", "/environment", "name"},
			{"view", "view_id", "/views", "title"},
			{"build-template", "build_template_id", "/templates", "name"},
		} {
			if !cmd.Flags().Changed(ref.flag) {
				continue
			}
			arg := cmd.Flags().Lookup(ref.flag).Value.String()
			id, err := resolveName(cmd, fmt.Sprintf("/project/%d%s", pid, ref.path), ref.nameKey, arg)
			if err != nil {
				return nil, err
			}
			m[ref.key] = id
		}
		return m, nil
	}

	create := &cobra.Command{
		Use:   "create",
		Short: "Create a template",
		RunE: func(cmd *cobra.Command, _ []string) error {
			pid, err := projectID(cmd)
			if err != nil {
				return err
			}
			m, err := body(cmd, pid)
			if err != nil {
				return exit(exitUsage, err)
			}
			if _, ok := m["project_id"]; !ok {
				m["project_id"] = pid
			}
			var t apiclient.Template
			if err := rt(cmd).api.Post(fmt.Sprintf("/project/%d/templates", pid), m, &t); err != nil {
				return mapAPIError(err)
			}
			return rt(cmd).printer.Print(t, templateColumns())
		},
	}

	update := &cobra.Command{
		Use:   "update <id|name>",
		Short: "Update a template: passed flags and --data fields replace current values",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pid, err := projectID(cmd)
			if err != nil {
				return err
			}
			cur, err := findItem(cmd, fmt.Sprintf("/project/%d/templates", pid), "name", args[0])
			if err != nil {
				return err
			}
			over, err := body(cmd, pid)
			if err != nil {
				return exit(exitUsage, err)
			}
			merge(cur, over)
			cur["project_id"] = pid
			id, _ := cur["id"].(float64)
			if err := rt(cmd).api.Put(fmt.Sprintf("/project/%d/templates/%d", pid, int(id)), cur, nil); err != nil {
				return mapAPIError(err)
			}
			var t apiclient.Template
			raw, _ := json.Marshal(cur)
			_ = json.Unmarshal(raw, &t)
			return rt(cmd).printer.Print(t, templateColumns())
		},
	}

	del := &cobra.Command{
		Use:   "delete <id|name>",
		Short: "Delete a template",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pid, err := projectID(cmd)
			if err != nil {
				return err
			}
			tpl, err := resolveTemplate(cmd, pid, args[0])
			if err != nil {
				return err
			}
			if err := confirm(cmd, fmt.Sprintf("Delete template %q (id %d)?", tpl.Name, tpl.ID)); err != nil {
				return err
			}
			if err := rt(cmd).api.Delete(fmt.Sprintf("/project/%d/templates/%d", pid, tpl.ID)); err != nil {
				return mapAPIError(err)
			}
			fmt.Fprintf(stderr, "template %d deleted\n", tpl.ID)
			return nil
		},
	}

	for _, c := range []*cobra.Command{create, update} {
		bind(c)
	}
	return []*cobra.Command{create, update, del}
}

var stderr io.Writer = os.Stderr
