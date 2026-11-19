package cmd

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Alg0rix/semaphore-cli/internal/apiclient"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

var readOnly = &mcp.ToolAnnotations{
	ReadOnlyHint:   true,
	IdempotentHint: true,
}

func newMcpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Serve the Semaphore API as MCP tools over stdio",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r := rt(cmd)
			srv := mcp.NewServer(&mcp.Implementation{Name: "semaphore-cli", Version: "0.1.0"}, nil)
			registerMcpTools(srv, r)
			return srv.Run(cmd.Context(), &mcp.StdioTransport{})
		},
	}
}

func mcpErr(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
	}
}

type projectArg struct {
	Project string `json:"project" jsonschema:"project id or name"`
}

type taskRefArg struct {
	Project string `json:"project" jsonschema:"project id or name"`
	TaskID  int    `json:"task_id" jsonschema:"task id"`
}

type resourceListArg struct {
	Project  string `json:"project" jsonschema:"project id or name (ignored for user and runner)"`
	Resource string `json:"resource" jsonschema:"one of: inventory, environment, schedule, key, repository, view, member, user, runner"`
}

type taskRunArg struct {
	Project   string `json:"project" jsonschema:"project id or name"`
	Template  string `json:"template" jsonschema:"template id or name"`
	Limit     string `json:"limit,omitempty" jsonschema:"ansible host limit"`
	Branch    string `json:"branch,omitempty" jsonschema:"override git branch"`
	Message   string `json:"message,omitempty" jsonschema:"task message"`
	Args      string `json:"args,omitempty" jsonschema:"extra CLI args as JSON array string"`
	DryRun    bool   `json:"dry_run,omitempty"`
	Wait      bool   `json:"wait,omitempty" jsonschema:"block until the task finishes and return the final task"`
	TimeoutMs int    `json:"timeout_ms,omitempty" jsonschema:"max wait milliseconds when wait is set (0 = no limit)"`
}

type taskWaitArg struct {
	Project   string `json:"project" jsonschema:"project id or name"`
	TaskID    int    `json:"task_id" jsonschema:"task id"`
	TimeoutMs int    `json:"timeout_ms,omitempty" jsonschema:"max wait milliseconds (0 = no limit)"`
}

type taskStopArg struct {
	Project string `json:"project" jsonschema:"project id or name"`
	TaskID  int    `json:"task_id" jsonschema:"task id"`
	Force   bool   `json:"force,omitempty" jsonschema:"kill the task process immediately"`
}

type mcpTask struct {
	ID         int    `json:"id"`
	TemplateID int    `json:"template_id"`
	Status     string `json:"status"`
	Message    string `json:"message,omitempty"`
	Limit      string `json:"limit,omitempty"`
	GitBranch  string `json:"git_branch,omitempty"`
	UserName   string `json:"user_name,omitempty"`
	Created    string `json:"created,omitempty"`
	Start      string `json:"start,omitempty"`
	End        string `json:"end,omitempty"`
	Version    string `json:"version,omitempty"`
	CommitHash string `json:"commit_hash,omitempty"`
	Debug      bool   `json:"debug,omitempty"`
	DryRun     bool   `json:"dry_run,omitempty"`
}

type mcpTemplate struct {
	ID             int    `json:"id"`
	ProjectID      int    `json:"project_id"`
	Name           string `json:"name"`
	App            string `json:"app"`
	Playbook       string `json:"playbook,omitempty"`
	Description    string `json:"description,omitempty"`
	Type           string `json:"type"`
	InventoryID    int    `json:"inventory_id"`
	RepositoryID   int    `json:"repository_id"`
	EnvironmentID  int    `json:"environment_id"`
	LastTaskID     int    `json:"last_task_id,omitempty"`
	LastTaskStatus string `json:"last_task_status,omitempty"`
}

func toMcpTask(t *apiclient.Task) mcpTask {
	o := mcpTask{
		ID: t.ID, TemplateID: t.TemplateID, Status: t.Status,
		Message: t.Message, Limit: t.Limit, GitBranch: t.GitBranch,
		UserName: t.UserName, Version: t.Version, CommitHash: t.CommitHash,
		Debug: t.Debug, DryRun: t.DryRun,
		Created: t.Created.Format(time.RFC3339),
	}
	if t.Start != nil {
		o.Start = t.Start.Format(time.RFC3339)
	}
	if t.End != nil {
		o.End = t.End.Format(time.RFC3339)
	}
	return o
}

func toMcpTemplate(t apiclient.Template) mcpTemplate {
	o := mcpTemplate{
		ID: t.ID, ProjectID: t.ProjectID, Name: t.Name, App: t.App,
		Playbook: t.Playbook, Description: t.Description, Type: t.Type,
		InventoryID: t.InventoryID, RepositoryID: t.RepositoryID,
		EnvironmentID: t.EnvironmentID,
	}
	if t.LastTask != nil {
		o.LastTaskID = t.LastTask.ID
		o.LastTaskStatus = t.LastTask.Status
	}
	return o
}

func (a projectArg) pid(r *runtime) (int, error) {
	if a.Project == "" {
		return 0, errors.New("project is required")
	}
	if n, err := strconv.Atoi(a.Project); err == nil {
		return n, nil
	}
	var projects []apiclient.Project
	if err := r.api.Get("/projects", &projects); err != nil {
		return 0, err
	}
	for _, p := range projects {
		if strings.EqualFold(p.Name, a.Project) {
			return p.ID, nil
		}
	}
	return 0, fmt.Errorf("project %q not found", a.Project)
}

func mcpWaitTask(ctx context.Context, r *runtime, pid, taskID int, timeout time.Duration) (*apiclient.Task, error) {
	deadline := time.Now().Add(timeout)
	for {
		var t apiclient.Task
		if err := r.api.Get(taskPath(pid, taskID, ""), &t); err != nil {
			return nil, err
		}
		if taskFinished(t.Status) {
			return &t, nil
		}
		if timeout > 0 && time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for task %d (still %s)", taskID, t.Status)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func mcpListResource(r *runtime, pid int, resource string) (any, error) {
	path := fmt.Sprintf("/project/%d/%s", pid, resource)
	switch resource {
	case "user", "runner":
		path = "/" + resource + "s"
	case "member":
		path = fmt.Sprintf("/project/%d/users", pid)
	case "schedule":
		path = fmt.Sprintf("/project/%d/schedules", pid)
	case "key":
		path = fmt.Sprintf("/project/%d/keys", pid)
	case "repository":
		path = fmt.Sprintf("/project/%d/repositories", pid)
	case "view":
		path = fmt.Sprintf("/project/%d/views", pid)
	case "inventory", "environment":
	default:
		return nil, fmt.Errorf("unknown resource %q: use inventory, environment, schedule, key, repository, view, member, user or runner", resource)
	}
	var out any
	if err := r.api.Get(path, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func registerMcpTools(srv *mcp.Server, r *runtime) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "info", Description: "Show Semaphore server version and info",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, apiclient.Info, error) {
		var info apiclient.Info
		if err := r.api.Get("/info", &info); err != nil {
			return mcpErr(err), info, nil
		}
		return nil, info, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "whoami", Description: "Show the user the API token belongs to",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, apiclient.User, error) {
		u, err := r.api.WhoAmI()
		if err != nil {
			return mcpErr(err), apiclient.User{}, nil
		}
		return nil, *u, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "project_list", Description: "List Semaphore projects",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, []apiclient.Project, error) {
		var out []apiclient.Project
		if err := r.api.Get("/projects", &out); err != nil {
			return mcpErr(err), out, nil
		}
		return nil, out, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "template_list", Description: "List templates of a project",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in projectArg) (*mcp.CallToolResult, []mcpTemplate, error) {
		var templates []apiclient.Template
		pid, err := in.pid(r)
		if err != nil {
			return mcpErr(err), nil, nil
		}
		if err := r.api.Get(fmt.Sprintf("/project/%d/templates", pid), &templates); err != nil {
			return mcpErr(err), nil, nil
		}
		out := make([]mcpTemplate, len(templates))
		for i, t := range templates {
			out[i] = toMcpTemplate(t)
		}
		return nil, out, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "resource_list", Description: "List a project or instance resource",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in resourceListArg) (*mcp.CallToolResult, any, error) {
		pid := 0
		switch in.Resource {
		case "user", "runner":
		default:
			var err error
			if pid, err = (projectArg{Project: in.Project}).pid(r); err != nil {
				return mcpErr(err), nil, nil
			}
		}
		out, err := mcpListResource(r, pid, in.Resource)
		if err != nil {
			return mcpErr(err), out, nil
		}
		return nil, out, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "task_run", Description: "Start a task from a template; optionally wait for the result",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in taskRunArg) (*mcp.CallToolResult, mcpTask, error) {
		var task apiclient.Task
		pid, err := (projectArg{Project: in.Project}).pid(r)
		if err != nil {
			return mcpErr(err), mcpTask{}, nil
		}
		req := apiclient.TaskRequest{
			TemplateName: in.Template,
			Limit:        in.Limit,
			GitBranch:    in.Branch,
			Message:      in.Message,
			Arguments:    in.Args,
			DryRun:       in.DryRun,
		}
		if id, err := strconv.Atoi(in.Template); err == nil {
			req.TemplateID = id
			req.TemplateName = ""
		}
		if err := r.api.Post(fmt.Sprintf("/project/%d/tasks", pid), req, &task); err != nil {
			return mcpErr(err), mcpTask{}, nil
		}
		if in.Wait {
			t, err := mcpWaitTask(ctx, r, pid, task.ID, time.Duration(in.TimeoutMs)*time.Millisecond)
			if err != nil {
				return mcpErr(err), mcpTask{}, nil
			}
			task = *t
		}
		return nil, toMcpTask(&task), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "task_wait", Description: "Wait for a task to finish; returns its final status",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in taskWaitArg) (*mcp.CallToolResult, mcpTask, error) {
		pid, err := (projectArg{Project: in.Project}).pid(r)
		if err != nil {
			return mcpErr(err), mcpTask{}, nil
		}
		t, err := mcpWaitTask(ctx, r, pid, in.TaskID, time.Duration(in.TimeoutMs)*time.Millisecond)
		if err != nil {
			return mcpErr(err), mcpTask{}, nil
		}
		return nil, toMcpTask(t), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "task_list", Description: "List recent tasks of a project, newest first",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in projectArg) (*mcp.CallToolResult, []mcpTask, error) {
		var tasks []apiclient.Task
		pid, err := in.pid(r)
		if err != nil {
			return mcpErr(err), nil, nil
		}
		if err := r.api.Get(fmt.Sprintf("/project/%d/tasks/last", pid), &tasks); err != nil {
			return mcpErr(err), nil, nil
		}
		out := make([]mcpTask, len(tasks))
		for i := range tasks {
			out[i] = toMcpTask(&tasks[i])
		}
		return nil, out, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "task_get", Description: "Show one task",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in taskRefArg) (*mcp.CallToolResult, mcpTask, error) {
		var task apiclient.Task
		pid, err := (projectArg{Project: in.Project}).pid(r)
		if err != nil {
			return mcpErr(err), mcpTask{}, nil
		}
		if err := r.api.Get(taskPath(pid, in.TaskID, ""), &task); err != nil {
			return mcpErr(err), mcpTask{}, nil
		}
		return nil, toMcpTask(&task), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "task_logs", Description: "Print the output lines of a task",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in taskRefArg) (*mcp.CallToolResult, []string, error) {
		pid, err := (projectArg{Project: in.Project}).pid(r)
		if err != nil {
			return mcpErr(err), nil, nil
		}
		var lines []apiclient.TaskOutput
		if err := r.api.Get(taskPath(pid, in.TaskID, "/output"), &lines); err != nil {
			return mcpErr(err), nil, nil
		}
		out := make([]string, len(lines))
		for i, l := range lines {
			out[i] = l.Output
		}
		return nil, out, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "task_stop", Description: "Stop a running task",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in taskStopArg) (*mcp.CallToolResult, string, error) {
		pid, err := (projectArg{Project: in.Project}).pid(r)
		if err != nil {
			return mcpErr(err), "", nil
		}
		body := map[string]bool{}
		if in.Force {
			body["force"] = true
		}
		if err := r.api.Post(taskPath(pid, in.TaskID, "/stop"), body, nil); err != nil {
			return mcpErr(err), "", nil
		}
		return nil, fmt.Sprintf("task %d stop requested", in.TaskID), nil
	})

	for _, v := range []struct{ name, suffix, desc string }{
		{"task_confirm", "/confirm", "let a task waiting for confirmation proceed"},
		{"task_reject", "/reject", "refuse a task waiting for confirmation"},
	} {
		v := v
		mcp.AddTool(srv, &mcp.Tool{
			Name: v.name, Description: "Task action: " + v.desc,
		}, func(ctx context.Context, _ *mcp.CallToolRequest, in taskRefArg) (*mcp.CallToolResult, string, error) {
			pid, err := (projectArg{Project: in.Project}).pid(r)
			if err != nil {
				return mcpErr(err), "", nil
			}
			if err := r.api.Post(taskPath(pid, in.TaskID, v.suffix), map[string]any{}, nil); err != nil {
				return mcpErr(err), "", nil
			}
			return nil, fmt.Sprintf("task %d %s done", in.TaskID, v.name[5:]), nil
		})
	}
}
