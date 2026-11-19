package cmd

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/Alg0rix/semaphore-cli/internal/apiclient"
	"github.com/Alg0rix/semaphore-cli/internal/output"

	"github.com/spf13/cobra"
)

type resource struct {
	kind    string
	short   string
	project bool // project-scoped vs instance-wide
	path    string
	columns []output.Column
	newList func() any // pointer to slice for json.Unmarshal
	newItem func() any // pointer to one item for json.Unmarshal
	index   func(v any, id string) (any, error)
	// nameKey is the JSON field matched for name lookups ("name", "title"…).
	nameKey string
	// fixup adjusts a merged request body before it is sent, for
	// resources whose write schema conflicts with their read schema.
	fixup func(body, over map[string]any)
	// optionalProject allows -p on a global resource for writes that
	// have a project-scoped variant (runners).
	optionalProject bool
	// createPath overrides the POST path when set.
	createPath func(pid int) string
}

func res[T any](kind, short string, project bool, path string, columns []output.Column,
	name func(T) string, id func(T) int) resource {
	return resource{
		kind: kind, short: short, project: project, path: path, columns: columns,
		nameKey: "name",
		newList: func() any { return &[]T{} },
		newItem: func() any { return new(T) },
		index: func(v any, want string) (any, error) {
			items := *(v.(*[]T))
			if n, err := strconv.Atoi(want); err == nil {
				for _, it := range items {
					if id(it) == n {
						return it, nil
					}
				}
				return nil, exit(exitNotFound, fmt.Errorf("%s %q not found", kind, want))
			}
			for _, it := range items {
				if strings.EqualFold(name(it), want) {
					return it, nil
				}
			}
			return nil, exit(exitNotFound, fmt.Errorf("%s %q not found", kind, want))
		},
	}
}

func col[T any, R any](header string, get func(T) R) output.Column {
	return output.Column{Header: header, Value: func(v any) string {
		return outputValue(get(v.(T)))
	}}
}

func outputValue(v any) string {
	switch t := v.(type) {
	case nil:
		return "-"
	case string:
		return t
	case time.Time:
		return t.Format("2006-01-02 15:04:05")
	case *time.Time:
		if t == nil {
			return "-"
		}
		return t.Format("2006-01-02 15:04:05")
	default:
		return fmt.Sprint(v)
	}
}

func (r resource) endpoint(projectID int) string {
	if r.project {
		return fmt.Sprintf("/project/%d%s", projectID, r.path)
	}
	return r.path
}

func (r resource) newCmd() *cobra.Command {
	cmd := &cobra.Command{Use: r.kind, Short: r.short}

	list := &cobra.Command{
		Use:   "list",
		Short: "List " + r.kind + "s",
		RunE: func(cmd *cobra.Command, _ []string) error {
			rt := rt(cmd)
			pid := 0
			if r.project {
				var err error
				if pid, err = projectID(cmd); err != nil {
					return err
				}
			}
			out := r.newList()
			if err := rt.api.Get(r.endpoint(pid), out); err != nil {
				return mapAPIError(err)
			}
			return rt.printer.Print(reflect.ValueOf(out).Elem().Interface(), r.columns)
		},
	}

	get := &cobra.Command{
		Use:   "get <id|name>",
		Short: "Show one " + r.kind,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rt := rt(cmd)
			pid := 0
			if r.project {
				var err error
				if pid, err = projectID(cmd); err != nil {
					return err
				}
			}
			out := r.newList()
			if err := rt.api.Get(r.endpoint(pid), out); err != nil {
				return mapAPIError(err)
			}
			item, err := r.index(out, args[0])
			if err != nil {
				return err
			}
			return rt.printer.Print(item, r.columns)
		},
	}
	cmd.AddCommand(list, get)
	r.addCrud(cmd)
	return cmd
}

func resourceCommands() []*cobra.Command {
	return []*cobra.Command{
		res[apiclient.Inventory]("inventory", "Inventories", true, "/inventory", []output.Column{
			col[apiclient.Inventory]("ID", func(i apiclient.Inventory) int { return i.ID }),
			col[apiclient.Inventory]("NAME", func(i apiclient.Inventory) string { return i.Name }),
			col[apiclient.Inventory]("TYPE", func(i apiclient.Inventory) string { return i.Type }),
		}, func(i apiclient.Inventory) string { return i.Name }, func(i apiclient.Inventory) int { return i.ID }).newCmd(),

		res[apiclient.Environment]("environment", "Environments (variable groups)", true, "/environment", []output.Column{
			col[apiclient.Environment]("ID", func(e apiclient.Environment) int { return e.ID }),
			col[apiclient.Environment]("NAME", func(e apiclient.Environment) string { return e.Name }),
		}, func(e apiclient.Environment) string { return e.Name }, func(e apiclient.Environment) int { return e.ID }).newCmd(),

		res[apiclient.Schedule]("schedule", "Schedules", true, "/schedules", []output.Column{
			col[apiclient.Schedule]("ID", func(s apiclient.Schedule) int { return s.ID }),
			col[apiclient.Schedule]("NAME", func(s apiclient.Schedule) string { return s.Name }),
			col[apiclient.Schedule]("CRON", func(s apiclient.Schedule) string { return s.CronFormat }),
			col[apiclient.Schedule]("ACTIVE", func(s apiclient.Schedule) bool { return s.Active }),
			col[apiclient.Schedule]("TEMPLATE", func(s apiclient.Schedule) int { return s.TemplateID }),
		}, func(s apiclient.Schedule) string { return s.Name }, func(s apiclient.Schedule) int { return s.ID }).newCmd(),

		res[apiclient.AccessKey]("key", "Access keys", true, "/keys", []output.Column{
			col[apiclient.AccessKey]("ID", func(k apiclient.AccessKey) int { return k.ID }),
			col[apiclient.AccessKey]("NAME", func(k apiclient.AccessKey) string { return k.Name }),
			col[apiclient.AccessKey]("TYPE", func(k apiclient.AccessKey) string { return k.Type }),
		}, func(k apiclient.AccessKey) string { return k.Name }, func(k apiclient.AccessKey) int { return k.ID }).newCmd(),

		res[apiclient.Repository]("repository", "Repositories", true, "/repositories", []output.Column{
			col[apiclient.Repository]("ID", func(r apiclient.Repository) int { return r.ID }),
			col[apiclient.Repository]("NAME", func(r apiclient.Repository) string { return r.Name }),
			col[apiclient.Repository]("URL", func(r apiclient.Repository) string { return r.GitURL }),
			col[apiclient.Repository]("BRANCH", func(r apiclient.Repository) string { return r.GitBranch }),
		}, func(r apiclient.Repository) string { return r.Name }, func(r apiclient.Repository) int { return r.ID }).newCmd(),

		viewCmd(),

		userCmd(),

		runnerCmd(),

		memberCmd(),
	}
}

// runnerCmd is the runner resource. Runners are global objects, but
// `runner create -p` registers a project runner through its project path.
func runnerCmd() *cobra.Command {
	r := res[apiclient.Runner]("runner", "Runners (admin only)", false, "/runners", []output.Column{
		col[apiclient.Runner]("ID", func(r apiclient.Runner) int { return r.ID }),
		col[apiclient.Runner]("NAME", func(r apiclient.Runner) string { return r.Name }),
		col[apiclient.Runner]("ACTIVE", func(r apiclient.Runner) bool { return r.Active }),
	}, func(r apiclient.Runner) string { return r.Name }, func(r apiclient.Runner) int { return r.ID })
	r.optionalProject = true
	r.createPath = func(pid int) string {
		if pid != 0 {
			return fmt.Sprintf("/project/%d/runners", pid)
		}
		return "/runners"
	}
	return r.newCmd()
}

func viewCmd() *cobra.Command {
	r := res[apiclient.View]("view", "Views", true, "/views", []output.Column{
		col[apiclient.View]("ID", func(v apiclient.View) int { return v.ID }),
		col[apiclient.View]("TITLE", func(v apiclient.View) string { return v.Title }),
		col[apiclient.View]("POSITION", func(v apiclient.View) int { return v.Position }),
	}, func(v apiclient.View) string { return v.Title }, func(v apiclient.View) int { return v.ID })
	r.nameKey = "title"
	return r.newCmd()
}

func userCmd() *cobra.Command {
	r := res[apiclient.User]("user", "Users (admin only)", false, "/users", []output.Column{
		col[apiclient.User]("ID", func(u apiclient.User) int { return u.ID }),
		col[apiclient.User]("USERNAME", func(u apiclient.User) string { return u.Username }),
		col[apiclient.User]("NAME", func(u apiclient.User) string { return u.Name }),
		col[apiclient.User]("EMAIL", func(u apiclient.User) string { return u.Email }),
		col[apiclient.User]("ADMIN", func(u apiclient.User) bool { return u.Admin }),
	}, func(u apiclient.User) string { return u.Username }, func(u apiclient.User) int { return u.ID })
	r.nameKey = "username"
	return r.newCmd()
}

// memberCmd is the member resource with a fixup for the role/roles
// conflict: the API rejects a body that carries both.
func memberCmd() *cobra.Command {
	r := res[apiclient.Member]("member", "Project members", true, "/users", []output.Column{
		col[apiclient.Member]("ID", func(m apiclient.Member) int { return m.ID }),
		col[apiclient.Member]("USERNAME", func(m apiclient.Member) string { return m.Username }),
		col[apiclient.Member]("NAME", func(m apiclient.Member) string { return m.Name }),
		col[apiclient.Member]("ROLE", func(m apiclient.Member) string { return m.Role }),
	}, func(m apiclient.Member) string { return m.Username }, func(m apiclient.Member) int { return m.ID })
	r.nameKey = "username"
	r.fixup = func(body, over map[string]any) {
		if _, ok := over["roles"]; ok {
			delete(body, "role")
		}
		if _, ok := over["role"]; ok {
			delete(body, "roles")
		}
	}
	return r.newCmd()
}
