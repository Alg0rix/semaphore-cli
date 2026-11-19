package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"semaphore-cli/internal/apiclient"
	"semaphore-cli/internal/output"

	"github.com/spf13/cobra"
)

func taskColumns() []output.Column {
	return []output.Column{
		col[apiclient.Task]("ID", func(t apiclient.Task) int { return t.ID }),
		col[apiclient.Task]("STATUS", func(t apiclient.Task) string { return t.Status }),
		col[apiclient.Task]("TEMPLATE", func(t apiclient.Task) int { return t.TemplateID }),
		col[apiclient.Task]("USER", func(t apiclient.Task) string { return t.UserName }),
		col[apiclient.Task]("MESSAGE", func(t apiclient.Task) string { return t.Message }),
		col[apiclient.Task]("START", func(t apiclient.Task) any { return t.Start }),
		col[apiclient.Task]("END", func(t apiclient.Task) any { return t.End }),
	}
}

func taskPath(pid, id int, suffix string) string {
	return fmt.Sprintf("/project/%d/tasks/%d%s", pid, id, suffix)
}

func getTask(cmd *cobra.Command, pid, id int) (*apiclient.Task, error) {
	var t apiclient.Task
	if err := rt(cmd).api.Get(taskPath(pid, id, ""), &t); err != nil {
		return nil, err
	}
	return &t, nil
}

func taskFinished(status string) bool {
	switch status {
	case "success", "error", "stopped":
		return true
	}
	return false
}

// waitTask polls a task until it leaves running-ish states. When follow is set
// it streams new output lines as they arrive.
func waitTask(cmd *cobra.Command, pid, taskID int, follow bool, timeout time.Duration) error {
	r := rt(cmd)
	deadline := time.Now().Add(timeout)
	printed := map[string]bool{}
	announce := !follow

	for {
		t, err := getTask(cmd, pid, taskID)
		if err != nil {
			return mapAPIError(err)
		}
		if follow {
			var lines []apiclient.TaskOutput
			if err := r.api.Get(taskPath(pid, taskID, "/output"), &lines); err == nil {
				for _, l := range lines {
					key := l.Time + l.Output
					if !printed[key] {
						fmt.Println(l.Output)
						printed[key] = true
					}
				}
			}
		} else if announce {
			fmt.Fprintf(stderr, "task %d: %s\n", t.ID, t.Status)
			announce = false
		}
		if taskFinished(t.Status) {
			if perr := r.printer.Print(t, taskColumns()); perr != nil {
				return perr
			}
			switch t.Status {
			case "error":
				return exit(exitFailed, fmt.Errorf("task %d failed", t.ID))
			case "stopped":
				return exit(exitStopped, fmt.Errorf("task %d was stopped", t.ID))
			}
			return nil
		}
		if timeout > 0 && time.Now().After(deadline) {
			return exit(exitTimeout, fmt.Errorf("timeout waiting for task %d", taskID))
		}
		time.Sleep(2 * time.Second)
	}
}

type runFlags struct {
	follow    bool
	wait      bool
	dryRun    bool
	diff      bool
	debug     bool
	limit     string
	branch    string
	message   string
	arguments string
	timeout   time.Duration
}

func (f *runFlags) bind(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&f.follow, "follow", "f", false, "follow the task log until it finishes")
	cmd.Flags().BoolVarP(&f.wait, "wait", "w", false, "wait silently; exit code follows task status")
	cmd.Flags().DurationVar(&f.timeout, "timeout", 0, "max wait time for --follow/--wait (default: no limit)")
	cmd.Flags().BoolVar(&f.dryRun, "dry-run", false, "ansible --check")
	cmd.Flags().BoolVar(&f.diff, "diff", false, "ansible --diff")
	cmd.Flags().BoolVar(&f.debug, "debug-task", false, "verbose ansible output")
	cmd.Flags().StringVar(&f.limit, "limit", "", "limit to hosts/group")
	cmd.Flags().StringVar(&f.branch, "branch", "", "override git branch")
	cmd.Flags().StringVar(&f.message, "message", "", "task message")
	cmd.Flags().StringVar(&f.arguments, "args", "", "extra CLI arguments (JSON array string)")
}

func runTask(cmd *cobra.Command, pid int, req apiclient.TaskRequest, f *runFlags) error {
	req.DryRun = f.dryRun
	req.Diff = f.diff
	req.Debug = f.debug
	req.Limit = f.limit
	req.GitBranch = f.branch
	req.Message = f.message
	req.Arguments = f.arguments

	var task apiclient.Task
	if err := rt(cmd).api.Post(fmt.Sprintf("/project/%d/tasks", pid), req, &task); err != nil {
		return mapAPIError(err)
	}
	fmt.Fprintf(stderr, "task %d started (%s)\n", task.ID, task.Status)
	if f.follow || f.wait {
		return waitTask(cmd, pid, task.ID, f.follow, f.timeout)
	}
	return rt(cmd).printer.Print(task, taskColumns())
}

func parseTaskID(arg string) (int, error) {
	id, err := strconv.Atoi(arg)
	if err != nil || id <= 0 {
		return 0, exit(exitUsage, fmt.Errorf("invalid task id %q", arg))
	}
	return id, nil
}

func newTaskCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "task", Short: "Inspect and manage tasks"}

	list := &cobra.Command{
		Use:   "list",
		Short: "List recent tasks of the project, newest first",
		RunE: func(cmd *cobra.Command, _ []string) error {
			pid, err := projectID(cmd)
			if err != nil {
				return err
			}
			var tasks []apiclient.Task
			if err := rt(cmd).api.Get(fmt.Sprintf("/project/%d/tasks/last", pid), &tasks); err != nil {
				return mapAPIError(err)
			}
			return rt(cmd).printer.Print(tasks, taskColumns())
		},
	}

	get := &cobra.Command{
		Use:   "get <id>",
		Short: "Show one task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pid, err := projectID(cmd)
			if err != nil {
				return err
			}
			id, err := parseTaskID(args[0])
			if err != nil {
				return err
			}
			t, err := getTask(cmd, pid, id)
			if err != nil {
				return mapAPIError(err)
			}
			return rt(cmd).printer.Print(t, taskColumns())
		},
	}

	var followLogs bool
	logs := &cobra.Command{
		Use:   "logs <id>",
		Short: "Print the log of a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pid, err := projectID(cmd)
			if err != nil {
				return err
			}
			id, err := parseTaskID(args[0])
			if err != nil {
				return err
			}
			if followLogs {
				return waitTask(cmd, pid, id, true, 0)
			}
			var lines []apiclient.TaskOutput
			if err := rt(cmd).api.Get(taskPath(pid, id, "/output"), &lines); err != nil {
				return mapAPIError(err)
			}
			if rt(cmd).printer.Format == "json" || rt(cmd).printer.Format == "yaml" {
				return rt(cmd).printer.Print(lines, nil)
			}
			for _, l := range lines {
				fmt.Println(l.Output)
			}
			return nil
		},
	}
	logs.Flags().BoolVarP(&followLogs, "follow", "f", false, "stream new output until the task finishes")

	var waitTimeout time.Duration
	wait := &cobra.Command{
		Use:   "wait <id>",
		Short: "Wait silently until a task finishes; exit code follows its status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pid, err := projectID(cmd)
			if err != nil {
				return err
			}
			id, err := parseTaskID(args[0])
			if err != nil {
				return err
			}
			return waitTask(cmd, pid, id, false, waitTimeout)
		},
	}
	wait.Flags().DurationVar(&waitTimeout, "timeout", 0, "max wait time (default: no limit)")

	var force bool
	stop := &cobra.Command{
		Use:   "stop <id>",
		Short: "Stop a running task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pid, err := projectID(cmd)
			if err != nil {
				return err
			}
			id, err := parseTaskID(args[0])
			if err != nil {
				return err
			}
			body := map[string]bool{}
			if force {
				body["force"] = true
			}
			if err := rt(cmd).api.Post(taskPath(pid, id, "/stop"), body, nil); err != nil {
				return mapAPIError(err)
			}
			fmt.Fprintf(stderr, "task %d stop requested\n", id)
			return nil
		},
	}
	stop.Flags().BoolVar(&force, "force", false, "kill the task process immediately")

	for _, a := range []struct{ name, verb, suffix string }{
		{"confirm", "confirmed", "/confirm"},
		{"reject", "rejected", "/reject"},
	} {
		a := a
		cmd.AddCommand(&cobra.Command{
			Use:   a.name + " <id>",
			Short: title(a.name) + " a task waiting for confirmation",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				pid, err := projectID(cmd)
				if err != nil {
					return err
				}
				id, err := parseTaskID(args[0])
				if err != nil {
					return err
				}
				if err := rt(cmd).api.Post(taskPath(pid, id, a.suffix), map[string]any{}, nil); err != nil {
					return mapAPIError(err)
				}
				fmt.Fprintf(stderr, "task %d %s\n", id, a.verb)
				return nil
			},
		})
	}

	var rf runFlags
	run := &cobra.Command{
		Use:   "run <id|name>",
		Short: "Start a task from a template (name resolved by the server)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pid, err := projectID(cmd)
			if err != nil {
				return err
			}
			req := apiclient.TaskRequest{TemplateName: args[0]}
			if id, err := strconv.Atoi(args[0]); err == nil {
				req.TemplateID = id
				req.TemplateName = ""
			}
			return runTask(cmd, pid, req, &rf)
		},
	}
	rf.bind(run)

	var rerunFollow bool
	rerun := &cobra.Command{
		Use:   "rerun <id>",
		Short: "Start a new task with the parameters of an existing one",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pid, err := projectID(cmd)
			if err != nil {
				return err
			}
			id, err := parseTaskID(args[0])
			if err != nil {
				return err
			}
			old, err := getTask(cmd, pid, id)
			if err != nil {
				return mapAPIError(err)
			}
			var task apiclient.Task
			err = rt(cmd).api.Post(fmt.Sprintf("/project/%d/tasks", pid), apiclient.TaskRequest{
				TemplateID: old.TemplateID,
				Debug:      old.Debug,
				DryRun:     old.DryRun,
				Diff:       old.Diff,
				Limit:      old.Limit,
				GitBranch:  old.GitBranch,
				Message:    old.Message,
			}, &task)
			if err != nil {
				return mapAPIError(err)
			}
			fmt.Fprintf(stderr, "task %d started (rerun of %d)\n", task.ID, id)
			if rerunFollow {
				return waitTask(cmd, pid, task.ID, true, 0)
			}
			return rt(cmd).printer.Print(task, taskColumns())
		},
	}
	rerun.Flags().BoolVarP(&rerunFollow, "follow", "f", false, "follow the new task log")

	del := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a task and its output",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pid, err := projectID(cmd)
			if err != nil {
				return err
			}
			id, err := parseTaskID(args[0])
			if err != nil {
				return err
			}
			if err := confirm(cmd, fmt.Sprintf("Delete task %d and its output?", id)); err != nil {
				return err
			}
			if err := rt(cmd).api.Delete(taskPath(pid, id, "")); err != nil {
				return mapAPIError(err)
			}
			fmt.Fprintf(stderr, "task %d deleted\n", id)
			return nil
		},
	}

	cmd.AddCommand(list, get, run, logs, wait, stop, rerun, del)
	return cmd
}

func title(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
