package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	EnvURL      = "SEMAPHORE_CLI_URL"
	EnvToken    = "SEMAPHORE_CLI_TOKEN"
	EnvContext  = "SEMAPHORE_CLI_CONTEXT"
	EnvProject  = "SEMAPHORE_CLI_PROJECT"
	EnvOutput   = "SEMAPHORE_CLI_OUTPUT"
	EnvInsecure = "SEMAPHORE_CLI_INSECURE"
)

type Context struct {
	URL      string `yaml:"url,omitempty"`
	Token    string `yaml:"token,omitempty"`
	Project  string `yaml:"project,omitempty"`
	Insecure bool   `yaml:"insecure,omitempty"`
	CACert   string `yaml:"ca_cert,omitempty"`
}

type File struct {
	Current  string              `yaml:"current"`
	Contexts map[string]*Context `yaml:"contexts"`
}

type Flags struct {
	URL      string
	Token    string
	Context  string
	Project  string
	Output   string
	Insecure bool
	CACert   string
	Timeout  time.Duration
}

type Effective struct {
	ContextName string
	Context
	Output  string
	Timeout time.Duration
}

var ErrUnknownContext = errors.New("unknown context")

func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".semaphore-cli", "config.yaml"), nil
}

func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &File{Contexts: map[string]*Context{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if f.Contexts == nil {
		f.Contexts = map[string]*Context{}
	}
	return &f, nil
}

func (f *File) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(f)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func Resolve(f Flags, getenv func(string) string, file *File) (Effective, error) {
	name := firstNonEmpty(f.Context, getenv(EnvContext), file.Current, "default")
	ctx, ok := file.Contexts[name]
	if !ok && (f.Context != "" || getenv(EnvContext) != "") {
		return Effective{}, fmt.Errorf("%w %q", ErrUnknownContext, name)
	}
	var c Context
	if ok {
		c = *ctx
	}

	insecure := f.Insecure || getenv(EnvInsecure) == "true" || c.Insecure
	return Effective{
		ContextName: name,
		Context: Context{
			URL:      firstNonEmpty(f.URL, getenv(EnvURL), c.URL),
			Token:    firstNonEmpty(f.Token, getenv(EnvToken), c.Token),
			Project:  firstNonEmpty(f.Project, getenv(EnvProject), c.Project),
			Insecure: insecure,
			CACert:   firstNonEmpty(f.CACert, c.CACert),
		},
		Output:  firstNonEmpty(f.Output, getenv(EnvOutput), "table"),
		Timeout: f.Timeout,
	}, nil
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}
