package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// bodyFlags carries a JSON request body from --data or --file.
type bodyFlags struct {
	data string
	file string
}

func (b *bodyFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&b.data, "data", "", "JSON request body")
	cmd.Flags().StringVar(&b.file, "file", "", "read JSON body from a file ('-' for stdin)")
}

func (b *bodyFlags) read(in io.Reader) (map[string]any, error) {
	var raw []byte
	var err error
	switch {
	case b.data != "":
		raw = []byte(b.data)
	case b.file == "-":
		raw, err = io.ReadAll(in)
	case b.file != "":
		raw, err = os.ReadFile(b.file)
	default:
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("invalid JSON body: %w", err)
	}
	return m, nil
}

// merge overlays src onto dst, recursively for nested maps.
func merge(dst, src map[string]any) {
	for k, v := range src {
		if sm, ok := v.(map[string]any); ok {
			if dm, ok := dst[k].(map[string]any); ok {
				merge(dm, sm)
				continue
			}
		}
		dst[k] = v
	}
}

// jsonMap converts a typed item into a JSON-shaped map.
func jsonMap(v any) (map[string]any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// findItem fetches a list endpoint and returns the item whose id or
// nameKey field matches arg.
func findItem(cmd *cobra.Command, path, nameKey, arg string) (map[string]any, error) {
	var items []map[string]any
	if err := rt(cmd).api.Get(path, &items); err != nil {
		return nil, mapAPIError(err)
	}
	id, numErr := strconv.Atoi(arg)
	for _, it := range items {
		if numErr == nil {
			if v, _ := it["id"].(float64); int(v) == id {
				return it, nil
			}
			continue
		}
		if name, _ := it[nameKey].(string); strings.EqualFold(name, arg) {
			return it, nil
		}
	}
	return nil, exit(exitNotFound, fmt.Errorf("%s %q not found", nameKey, arg))
}

// resolveName maps an id-or-name argument to a numeric id by scanning a list
// endpoint. nameKey is the field matched against arg ("name" or "title").
func resolveName(cmd *cobra.Command, path, nameKey, arg string) (int, error) {
	if arg == "" {
		return 0, nil
	}
	if n, err := strconv.Atoi(arg); err == nil {
		return n, nil
	}
	it, err := findItem(cmd, path, nameKey, arg)
	if err != nil {
		return 0, err
	}
	id, _ := it["id"].(float64)
	return int(id), nil
}

// flagSet values into body only when the user passed them.
func flagInto(cmd *cobra.Command, body map[string]any, flag, key string) {
	if !cmd.Flags().Changed(flag) {
		return
	}
	f := cmd.Flags().Lookup(flag)
	switch f.Value.Type() {
	case "bool":
		v, _ := cmd.Flags().GetBool(flag)
		body[key] = v
	case "int":
		v, _ := cmd.Flags().GetInt(flag)
		body[key] = v
	default:
		body[key] = f.Value.String()
	}
}

// crud adds create/update/delete to a resource command. Bodies come from
// --data/--file merged over flagSet fields and, for update, the fetched item.
func (r resource) addCrud(cmd *cobra.Command) {
	var bf bodyFlags

	pidOf := func(cmd *cobra.Command) (int, error) {
		if r.project {
			return projectID(cmd)
		}
		if r.optionalProject && rt(cmd).cfg.Project != "" {
			return projectID(cmd)
		}
		return 0, nil
	}

	create := &cobra.Command{
		Use:   "create",
		Short: "Create a " + r.kind + " from --data/--file JSON",
		RunE: func(cmd *cobra.Command, _ []string) error {
			rt := rt(cmd)
			pid, err := pidOf(cmd)
			if err != nil {
				return err
			}
			body, err := bf.read(rt.in)
			if err != nil {
				return exit(exitUsage, err)
			}
			if pid != 0 {
				if _, ok := body["project_id"]; !ok {
					body["project_id"] = pid
				}
			}
			path := r.endpoint(pid)
			if r.createPath != nil {
				path = r.createPath(pid)
			}
			out := r.newItem()
			if err := rt.api.Post(path, body, out); err != nil {
				return mapAPIError(err)
			}
			return rt.printer.Print(out, r.columns)
		},
	}

	update := &cobra.Command{
		Use:   "update <id|name>",
		Short: "Update a " + r.kind + ": fields in --data/--file replace the current values",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rt := rt(cmd)
			pid, err := pidOf(cmd)
			if err != nil {
				return err
			}
			body, err := findItem(cmd, r.endpoint(pid), r.nameKey, args[0])
			if err != nil {
				return err
			}
			over, err := bf.read(rt.in)
			if err != nil {
				return exit(exitUsage, err)
			}
			merge(body, over)
			if r.project {
				body["project_id"] = pid
			}
			if r.fixup != nil {
				r.fixup(body, over)
			}
			id, _ := body["id"].(float64)
			if err := rt.api.Put(fmt.Sprintf("%s/%d", r.endpoint(pid), int(id)), body, nil); err != nil {
				return mapAPIError(err)
			}
			raw, _ := json.Marshal(body)
			out := r.newItem()
			_ = json.Unmarshal(raw, out)
			return rt.printer.Print(out, r.columns)
		},
	}

	del := &cobra.Command{
		Use:   "delete <id|name>",
		Short: "Delete a " + r.kind,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rt := rt(cmd)
			pid, err := pidOf(cmd)
			if err != nil {
				return err
			}
			m, err := findItem(cmd, r.endpoint(pid), r.nameKey, args[0])
			if err != nil {
				return err
			}
			id, _ := m["id"].(float64)
			if err := confirm(cmd, fmt.Sprintf("Delete %s %q (id %d)?", r.kind, args[0], int(id))); err != nil {
				return err
			}
			if err := rt.api.Delete(fmt.Sprintf("%s/%d", r.endpoint(pid), int(id))); err != nil {
				return mapAPIError(err)
			}
			fmt.Fprintf(stderr, "%s %d deleted\n", r.kind, int(id))
			return nil
		},
	}

	for _, c := range []*cobra.Command{create, update} {
		bf.bind(c)
	}
	cmd.AddCommand(create, update, del)
}
