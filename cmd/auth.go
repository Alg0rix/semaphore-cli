package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"semaphore-cli/internal/apiclient"
	"semaphore-cli/internal/config"
	"semaphore-cli/internal/output"

	"github.com/spf13/cobra"
)

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Log in to a server and manage the saved token",
	}

	var withToken bool
	var username, password, tokenName, contextName string

	login := &cobra.Command{
		Use:   "login",
		Short: "Save a server and API token as a context",
		Example: `  semaphore-cli auth login --url https://semaphore.example.com
  semaphore-cli auth login --url https://semaphore.example.com --user alice
  echo "$TOKEN" | semaphore-cli auth login --url https://semaphore.example.com --with-token`,
		Annotations: map[string]string{"skip-runtime": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if flagSet.URL == "" {
				return exit(exitUsage, errors.New("--url is required"))
			}
			path, err := config.Path()
			if err != nil {
				return err
			}
			file, err := config.Load(path)
			if err != nil {
				return err
			}

			opts := apiclient.Options{
				URL:      flagSet.URL,
				Insecure: flagSet.Insecure,
				CACert:   flagSet.CACert,
				Timeout:  flagSet.Timeout,
			}
			var token string
			switch {
			case flagSet.Token != "":
				token = flagSet.Token
			case withToken:
				line, err := bufio.NewReader(os.Stdin).ReadString('\n')
				if err != nil {
					return exit(exitError, fmt.Errorf("read token from stdin: %w", err))
				}
				token = strings.TrimSpace(line)
			default:
				if username == "" {
					fmt.Fprint(os.Stderr, "Username or email: ")
					line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
					username = strings.TrimSpace(line)
				}
				if password == "" {
					pw, err := readPassword("Password: ")
					if err != nil {
						return err
					}
					password = pw
				}
				sess, err := apiclient.Login(opts, username, password)
				if err != nil {
					return exit(exitAuth, fmt.Errorf("login failed: %w", err))
				}
				if tokenName == "" {
					tokenName = "semaphore-cli"
				}
				token, err = sess.CreateToken(tokenName)
				if err != nil {
					return exit(exitError, fmt.Errorf("create API token: %w", err))
				}
			}
			if token == "" {
				return exit(exitUsage, errors.New("empty token"))
			}

			name := contextName
			if name == "" {
				name = flagSet.Context
			}
			if name == "" {
				name = "default"
			}
			if file.Contexts == nil {
				file.Contexts = map[string]*config.Context{}
			}
			c := file.Contexts[name]
			if c == nil {
				c = &config.Context{}
				file.Contexts[name] = c
			}
			c.URL = flagSet.URL
			c.Token = token
			if flagSet.Insecure {
				c.Insecure = true
			}
			if flagSet.CACert != "" {
				c.CACert = flagSet.CACert
			}
			if file.Current == "" {
				file.Current = name
			}
			if err := file.Save(path); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Logged in to %s, context %q saved.\n", flagSet.URL, name)
			return nil
		},
	}
	login.Flags().BoolVar(&withToken, "with-token", false, "read API token from stdin")
	login.Flags().StringVar(&username, "user", "", "username or email for password login")
	login.Flags().StringVar(&password, "password", "", "password (insecure; prefer interactive prompt)")
	login.Flags().StringVar(&tokenName, "token-name", "", "name for the API token created by password login")
	login.Flags().StringVarP(&contextName, "name", "n", "", "context name to save (default: 'default')")

	status := &cobra.Command{
		Use:   "status",
		Short: "Show which server and token are in use",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r := rt(cmd)
			fmt.Fprintf(os.Stderr, "context: %s\nurl:     %s\n", r.cfg.ContextName, r.cfg.URL)
			if r.cfg.Token == "" {
				return exit(exitAuth, errors.New("no token configured"))
			}
			u, err := r.api.WhoAmI()
			if err != nil {
				return mapAPIError(err)
			}
			return r.printer.Print(u, []output.Column{
				{Header: "ID", Value: func(v any) string { return fmt.Sprint(v.(apiclient.User).ID) }},
				{Header: "USERNAME", Value: func(v any) string { return v.(apiclient.User).Username }},
				{Header: "NAME", Value: func(v any) string { return v.(apiclient.User).Name }},
				{Header: "EMAIL", Value: func(v any) string { return v.(apiclient.User).Email }},
				{Header: "ADMIN", Value: func(v any) string { return fmt.Sprint(v.(apiclient.User).Admin) }},
			})
		},
	}

	tokenCmd := &cobra.Command{
		Use:   "token",
		Short: "Print the API token in use",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r := rt(cmd)
			if r.cfg.Token == "" {
				return exit(exitAuth, errors.New("no token configured"))
			}
			fmt.Println(r.cfg.Token)
			return nil
		},
	}

	logout := &cobra.Command{
		Use:   "logout",
		Short: "Forget the token of the current context",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r := rt(cmd)
			c := r.cfgFile.Contexts[r.cfg.ContextName]
			if c == nil || c.Token == "" {
				return exit(exitError, fmt.Errorf("context %q has no saved token", r.cfg.ContextName))
			}
			c.Token = ""
			if err := r.cfgFile.Save(r.cfgPath); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Token removed from context %q.\n", r.cfg.ContextName)
			return nil
		},
	}

	cmd.AddCommand(login, status, tokenCmd, logout)
	return cmd
}

func readPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
