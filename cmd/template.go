package cmd

import (
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
	return cmd
}

var stderr io.Writer = os.Stderr
