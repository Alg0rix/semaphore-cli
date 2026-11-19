package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"semaphore-cli/internal/apiclient"
	"semaphore-cli/internal/config"
	"semaphore-cli/internal/output"

	"github.com/spf13/cobra"
)

const (
	exitError    = 1
	exitUsage    = 2
	exitFailed   = 3
	exitAuth     = 4
	exitNotFound = 5
	exitStopped  = 6
	exitTimeout  = 124
	exitCtrlC    = 130
)

type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string { return e.Err.Error() }
func (e *ExitError) Unwrap() error { return e.Err }

func exit(code int, err error) error {
	return &ExitError{Code: code, Err: err}
}

type runtime struct {
	flags   *config.Flags
	cfg     config.Effective
	cfgFile *config.File
	cfgPath string
	api     *apiclient.Client
	printer *output.Printer
	in      io.Reader
}

var flagSet config.Flags
var flagFields []string
var flagQuiet bool
var flagNoHeaders bool
var flagYes bool
var flagDebug bool

func Execute() {
	root := &cobra.Command{
		Use:   "semaphore-cli",
		Short: "Command-line client for a Semaphore UI server",
		Long: `semaphore-cli talks to a Semaphore UI server over its REST API.

Authenticate with 'semaphore-cli auth login' (saved in ~/.semaphore-cli/config.yaml),
or pass --url/--token, or set SEMAPHORE_CLI_URL/SEMAPHORE_CLI_TOKEN. Precedence:
flag > environment > saved context.

Exit codes: 0 ok, 1 error, 2 usage, 3 task failed, 4 auth failed,
5 not found, 6 task stopped, 124 wait timeout.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	pf := root.PersistentFlags()
	pf.StringVar(&flagSet.URL, "url", "", "server URL ($"+config.EnvURL+")")
	pf.StringVar(&flagSet.Token, "token", "", "API token ($"+config.EnvToken+")")
	pf.StringVar(&flagSet.Context, "context", "", "named context ($"+config.EnvContext+")")
	pf.StringVarP(&flagSet.Project, "project", "p", "", "project id or name ($"+config.EnvProject+")")
	pf.StringVarP(&flagSet.Output, "output", "o", "", "output format: table, json, yaml ($"+config.EnvOutput+")")
	pf.StringSliceVar(&flagFields, "fields", nil, "comma-separated fields for table output")
	pf.BoolVarP(&flagQuiet, "quiet", "q", false, "print only ids")
	pf.BoolVar(&flagNoHeaders, "no-headers", false, "omit table header")
	pf.BoolVarP(&flagYes, "yes", "y", false, "answer yes to confirmations")
	pf.BoolVar(&flagSet.Insecure, "insecure", false, "skip TLS verification ($"+config.EnvInsecure+")")
	pf.StringVar(&flagSet.CACert, "ca-cert", "", "PEM CA file for server certificate")
	pf.DurationVar(&flagSet.Timeout, "timeout", 0, "HTTP timeout (default 30s)")
	pf.BoolVar(&flagDebug, "debug", false, "trace HTTP requests to stderr")

	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		for c := cmd; c != nil; c = c.Parent() {
			if c.Annotations["skip-runtime"] == "true" {
				return nil
			}
		}
		r, err := newRuntime()
		if err != nil {
			return err
		}
		cmd.SetContext(contextWithRuntime(cmd.Context(), r))
		return nil
	}

	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return exit(exitUsage, fmt.Errorf("%s (see '%s --help')", err, cmd.CommandPath()))
	})

	root.AddCommand(
		newAuthCmd(), newContextCmd(), newInfoCmd(),
		newProjectCmd(), newTemplateCmd(), newTaskCmd(), newMcpCmd(), newSkillCmd(),
	)
	root.AddCommand(resourceCommands()...)

	if err := root.Execute(); err != nil {
		var ee *ExitError
		if errors.As(err, &ee) {
			fmt.Fprintln(os.Stderr, "semaphore-cli:", ee.Err)
			os.Exit(ee.Code)
		}
		fmt.Fprintln(os.Stderr, "semaphore-cli:", err)
		os.Exit(exitError)
	}
}

type ctxKey struct{}

func contextWithRuntime(ctx context.Context, r *runtime) context.Context {
	return context.WithValue(ctx, ctxKey{}, r)
}

func rt(cmd *cobra.Command) *runtime {
	return cmd.Context().Value(ctxKey{}).(*runtime)
}

func newRuntime() (*runtime, error) {
	path, err := config.Path()
	if err != nil {
		return nil, err
	}
	file, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	eff, err := config.Resolve(flagSet, os.Getenv, file)
	if err != nil {
		return nil, err
	}
	api, err := apiclient.New(apiclient.Options{
		URL:      eff.URL,
		Token:    eff.Token,
		Insecure: eff.Insecure,
		CACert:   eff.CACert,
		Timeout:  eff.Timeout,
	})
	if err != nil {
		return nil, err
	}
	if flagDebug {
		api.Debug = os.Stderr
	}
	return &runtime{
		flags:   &flagSet,
		cfg:     eff,
		cfgFile: file,
		cfgPath: path,
		api:     api,
		printer: output.New(os.Stdout, eff.Output, flagFields, flagQuiet, flagNoHeaders),
		in:      os.Stdin,
	}, nil
}

// projectID resolves -p/--project: numeric id or project name lookup.
func projectID(cmd *cobra.Command) (int, error) {
	r := rt(cmd)
	s := r.cfg.Project
	if s == "" {
		return 0, exit(exitUsage, errors.New("project required: pass -p, set SEMAPHORE_CLI_PROJECT, or save one in the context"))
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n, nil
	}
	var projects []apiclient.Project
	if err := r.api.Get("/projects", &projects); err != nil {
		return 0, mapAPIError(err)
	}
	for _, p := range projects {
		if strings.EqualFold(p.Name, s) {
			return p.ID, nil
		}
	}
	return 0, exit(exitNotFound, fmt.Errorf("project %q not found", s))
}

func mapAPIError(err error) error {
	var ee *ExitError
	if errors.As(err, &ee) {
		return err
	}
	switch {
	case errors.Is(err, apiclient.ErrUnauthorized):
		return exit(exitAuth, err)
	case errors.Is(err, apiclient.ErrNotFound):
		return exit(exitNotFound, err)
	default:
		return exit(exitError, err)
	}
}

func confirm(cmd *cobra.Command, prompt string) error {
	if flagYes {
		return nil
	}
	if !output.IsTerminal(os.Stdin) {
		return exit(exitUsage, errors.New("confirmation required: pass -y/--yes in non-interactive mode"))
	}
	fmt.Fprintf(os.Stderr, "%s [y/N] ", prompt)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return err
	}
	if strings.TrimSpace(strings.ToLower(line)) != "y" {
		return exit(exitError, errors.New("aborted"))
	}
	return nil
}
