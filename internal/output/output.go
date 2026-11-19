package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"gopkg.in/yaml.v3"
)

type Column struct {
	Header string
	Value  func(item any) string
}

type Printer struct {
	out       io.Writer
	Format    string
	Fields    []string
	Quiet     bool
	NoHeaders bool
}

func New(out io.Writer, format string, fields []string, quiet, noHeaders bool) *Printer {
	return &Printer{out: out, Format: format, Fields: fields, Quiet: quiet, NoHeaders: noHeaders}
}

func (p *Printer) Print(v any, cols []Column) error {
	switch p.Format {
	case "json":
		enc := json.NewEncoder(p.out)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	case "yaml":
		return yaml.NewEncoder(p.out).Encode(v)
	case "table", "":
		return p.table(v, cols)
	default:
		return fmt.Errorf("unknown output format %q: use table, json or yaml", p.Format)
	}
}

func (p *Printer) table(v any, cols []Column) error {
	items := toSlice(v)
	if p.Quiet {
		for _, it := range items {
			fmt.Fprintln(p.out, idOf(it))
		}
		return nil
	}
	if len(p.Fields) > 0 {
		cols = fieldColumns(p.Fields)
	}
	w := tabwriter.NewWriter(p.out, 0, 4, 2, ' ', 0)
	if !p.NoHeaders {
		var hdr []string
		for _, c := range cols {
			hdr = append(hdr, c.Header)
		}
		fmt.Fprintln(w, strings.Join(hdr, "\t"))
	}
	for _, it := range items {
		var row []string
		for _, c := range cols {
			row = append(row, c.Value(it))
		}
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	return w.Flush()
}

func toSlice(v any) []any {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Slice {
		return []any{rv.Interface()}
	}
	out := make([]any, rv.Len())
	for i := range out {
		it := rv.Index(i)
		for it.Kind() == reflect.Ptr || it.Kind() == reflect.Interface {
			if it.IsNil() {
				break
			}
			it = it.Elem()
		}
		out[i] = it.Interface()
	}
	return out
}

func idOf(it any) string {
	rv := reflect.Indirect(reflect.ValueOf(it))
	if f := rv.FieldByName("ID"); f.IsValid() {
		return fmt.Sprint(f.Interface())
	}
	if f := rv.FieldByName("UserID"); f.IsValid() {
		return fmt.Sprint(f.Interface())
	}
	return ""
}

func fieldColumns(fields []string) []Column {
	cols := make([]Column, len(fields))
	for i, name := range fields {
		cols[i] = Column{Header: strings.ToUpper(name), Value: func(it any) string {
			rv := reflect.Indirect(reflect.ValueOf(it))
			for _, part := range strings.Split(name, ".") {
				if rv.Kind() != reflect.Struct {
					return ""
				}
				f := rv.FieldByNameFunc(func(fn string) bool {
					return strings.EqualFold(fn, part) || jsonName(rv, fn) == part
				})
				if !f.IsValid() {
					return ""
				}
				rv = f
			}
			return valueString(rv.Interface())
		}}
	}
	return cols
}

func jsonName(rv reflect.Value, fieldName string) string {
	f, ok := rv.Type().FieldByName(fieldName)
	if !ok {
		return ""
	}
	tag := f.Tag.Get("json")
	return strings.Split(tag, ",")[0]
}

func valueString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case time.Time:
		return t.Format("2006-01-02 15:04:05")
	case *time.Time:
		if t == nil {
			return "-"
		}
		return t.Format("2006-01-02 15:04:05")
	case nil:
		return "-"
	default:
		return fmt.Sprint(v)
	}
}

func IsTerminal(f *os.File) bool {
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}
