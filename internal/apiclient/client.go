package apiclient

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"
)

const UserAgent = "semaphore-cli/0.1"

var (
	ErrUnauthorized = errors.New("authentication failed: invalid or expired token")
	ErrNotFound     = errors.New("not found")
)

type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string { return e.Message }

type Client struct {
	base  *url.URL
	token string
	http  *http.Client
	Debug io.Writer
}

type Options struct {
	URL      string
	Token    string
	Insecure bool
	CACert   string
	Timeout  time.Duration
}

func New(o Options) (*Client, error) {
	if o.URL == "" {
		return nil, errors.New("server URL is required: pass --url, set SEMCLI_URL, or select a context")
	}
	base, err := url.Parse(o.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid server URL %q: %w", o.URL, err)
	}
	if base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("invalid server URL %q: expected e.g. https://semaphore.example.com", o.URL)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if o.Insecure || o.CACert != "" {
		cfg := &tls.Config{InsecureSkipVerify: o.Insecure} //nolint:gosec // explicit user flag
		if o.CACert != "" {
			pem, err := os.ReadFile(o.CACert)
			if err != nil {
				return nil, fmt.Errorf("read CA cert: %w", err)
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(pem) {
				return nil, fmt.Errorf("no PEM certificates in %s", o.CACert)
			}
			cfg.RootCAs = pool
		}
		transport.TLSClientConfig = cfg
	}

	timeout := o.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &Client{base: base, token: o.Token, http: &http.Client{Transport: transport, Timeout: timeout}}, nil
}

// NewSessionClient returns a client that authenticates with cookies instead of
// a bearer token; used by password login.
func NewSessionClient(o Options) (*Client, error) {
	c, err := New(o)
	if err != nil {
		return nil, err
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	c.http.Jar = jar
	return c, nil
}

func (c *Client) api(path string) string {
	base := *c.base
	p := strings.TrimSuffix(base.Path, "/")
	base.Path = p + "/api" + path
	return base.String()
}

func (c *Client) Do(method, path string, body, out any) error {
	var rd io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(buf)
	}
	req, err := http.NewRequest(method, c.api(path), rd)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("User-Agent", UserAgent)

	if c.Debug != nil {
		fmt.Fprintf(c.Debug, "> %s %s\n", method, req.URL.Redacted())
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if c.Debug != nil {
		fmt.Fprintf(c.Debug, "< %d %s\n", resp.StatusCode, resp.Status)
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusNotFound:
		return fmt.Errorf("%s: %w", firstLine(string(data), "resource"), ErrNotFound)
	}
	if resp.StatusCode >= 400 {
		msg := strings.TrimSpace(string(data))
		if msg == "" {
			msg = resp.Status
		}
		return &Error{Status: resp.StatusCode, Message: fmt.Sprintf("server error: %s", msg)}
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, out)
}

func firstLine(s, fallback string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if s = strings.TrimSpace(s); s != "" {
		return s
	}
	return fallback
}

func (c *Client) Get(path string, out any) error        { return c.Do("GET", path, nil, out) }
func (c *Client) Post(path string, body, out any) error { return c.Do("POST", path, body, out) }
func (c *Client) Delete(path string) error              { return c.Do("DELETE", path, nil, nil) }

// Login authenticates with username/password and returns a session-cookie client.
func Login(o Options, user, password string) (*Client, error) {
	c, err := NewSessionClient(o)
	if err != nil {
		return nil, err
	}
	body := map[string]string{"auth": user, "password": password}
	if err := c.Do("POST", "/auth/login", body, nil); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Client) CreateToken(name string) (string, error) {
	var tok APIToken
	if err := c.Post("/user/tokens", map[string]string{"name": name}, &tok); err != nil {
		return "", err
	}
	if tok.ID == "" {
		return "", errors.New("server returned an empty API token")
	}
	return tok.ID, nil
}

func (c *Client) WhoAmI() (*User, error) {
	var u User
	if err := c.Get("/user/", &u); err != nil {
		return nil, err
	}
	return &u, nil
}
