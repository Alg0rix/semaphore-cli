package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// SkillsFS carries the embedded skills/ tree; main sets it.
var SkillsFS fs.FS

const skillName = "semaphore-cli"

func newSkillCmd() *cobra.Command {
	var path string

	cmd := &cobra.Command{
		Use:         "skill",
		Short:       "Manage the semaphore-cli agent skill",
		Annotations: map[string]string{"skip-runtime": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.PersistentFlags().StringVar(&path, "path", "", "skills directory to install into (default: all standard agent locations)")

	install := &cobra.Command{
		Use:   "install",
		Short: "Install the embedded agent skill for AI coding agents",
		Long: `Copies the bundled skills/` + skillName + `/ directory into the standard
skill locations (~/.agents/skills, ~/.claude/skills, ~/.config/devin/skills)
so agents like Claude Code, Devin, or Codex can discover it.

Use --path to install into a single directory instead, e.g. a
project-local .agents/skills.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			targets, err := skillTargets(path)
			if err != nil {
				return exit(exitError, err)
			}
			for _, t := range targets {
				dest := filepath.Join(t, skillName)
				if err := copySkillDir(dest); err != nil {
					return exit(exitError, fmt.Errorf("install into %s: %w", dest, err))
				}
				fmt.Fprintln(os.Stdout, dest)
			}
			return nil
		},
	}
	cmd.AddCommand(install)
	return cmd
}

func skillTargets(path string) ([]string, error) {
	if path != "" {
		return []string{path}, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return []string{
		filepath.Join(home, ".agents", "skills"),
		filepath.Join(home, ".claude", "skills"),
		filepath.Join(home, ".config", "devin", "skills"),
	}, nil
}

func copySkillDir(dest string) error {
	src := "skills/" + skillName
	return fs.WalkDir(SkillsFS, src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		out := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		data, err := fs.ReadFile(SkillsFS, p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		return os.WriteFile(out, data, 0o644)
	})
}
