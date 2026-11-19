package apiclient

import "time"

type Info struct {
	Version          string `json:"version"`
	Ansible          string `json:"ansible"`
	WebHost          string `json:"web_host"`
	UseRemoteRunners bool   `json:"use_remote_runner"`
	ScheduleTimezone string `json:"schedule_timezone"`
	GitClient        string `json:"git_client"`
}

type Project struct {
	ID              int       `json:"id" yaml:"id"`
	Name            string    `json:"name" yaml:"name"`
	Created         time.Time `json:"created" yaml:"created"`
	Alert           bool      `json:"alert" yaml:"alert"`
	MaxParallelTask int       `json:"max_parallel_tasks" yaml:"max_parallel_tasks"`
	Type            string    `json:"type" yaml:"type"`
}

type Template struct {
	ID            int    `json:"id" yaml:"id"`
	ProjectID     int    `json:"project_id" yaml:"project_id"`
	Name          string `json:"name" yaml:"name"`
	App           string `json:"app" yaml:"app"`
	Playbook      string `json:"playbook" yaml:"playbook"`
	Description   string `json:"description" yaml:"description"`
	InventoryID   int    `json:"inventory_id" yaml:"inventory_id"`
	RepositoryID  int    `json:"repository_id" yaml:"repository_id"`
	EnvironmentID int    `json:"environment_id" yaml:"environment_id"`
	Type          string `json:"type" yaml:"type"`
	LastTask      *Task  `json:"last_task" yaml:"last_task"`
}

type Task struct {
	ID         int        `json:"id" yaml:"id"`
	TemplateID int        `json:"template_id" yaml:"template_id"`
	Status     string     `json:"status" yaml:"status"`
	Message    string     `json:"message" yaml:"message"`
	Debug      bool       `json:"debug" yaml:"debug"`
	DryRun     bool       `json:"dry_run" yaml:"dry_run"`
	Diff       bool       `json:"diff" yaml:"diff"`
	Limit      string     `json:"limit" yaml:"limit"`
	GitBranch  string     `json:"git_branch" yaml:"git_branch"`
	UserID     int        `json:"user_id" yaml:"user_id"`
	UserName   string     `json:"user_name" yaml:"user_name"`
	Created    time.Time  `json:"created" yaml:"created"`
	Start      *time.Time `json:"start" yaml:"start"`
	End        *time.Time `json:"end" yaml:"end"`
	Version    string     `json:"version" yaml:"version"`
	CommitHash string     `json:"commit_hash" yaml:"commit_hash"`
	BuildTask  *Task      `json:"build_task" yaml:"build_task"`
}

type TaskOutput struct {
	TaskID int    `json:"task_id" yaml:"task_id"`
	Task   string `json:"task" yaml:"task"`
	Time   string `json:"time" yaml:"time"`
	Output string `json:"output" yaml:"output"`
}

type TaskRequest struct {
	TemplateID   int    `json:"template_id,omitempty"`
	TemplateName string `json:"template_name,omitempty"`
	Debug        bool   `json:"debug,omitempty"`
	DryRun       bool   `json:"dry_run,omitempty"`
	Diff         bool   `json:"diff,omitempty"`
	Playbook     string `json:"playbook,omitempty"`
	Environment  string `json:"environment,omitempty"`
	Limit        string `json:"limit,omitempty"`
	GitBranch    string `json:"git_branch,omitempty"`
	Message      string `json:"message,omitempty"`
	Arguments    string `json:"arguments,omitempty"`
	InventoryID  int    `json:"inventory_id,omitempty"`
}

type Inventory struct {
	ID           int    `json:"id" yaml:"id"`
	Name         string `json:"name" yaml:"name"`
	Type         string `json:"type" yaml:"type"`
	RepositoryID int    `json:"repository_id" yaml:"repository_id"`
	Inventory    string `json:"inventory" yaml:"inventory"`
}

type Environment struct {
	ID       int    `json:"id" yaml:"id"`
	Name     string `json:"name" yaml:"name"`
	Password string `json:"password" yaml:"password"`
	JSON     string `json:"json" yaml:"json"`
	ENV      string `json:"env" yaml:"env"`
}

type Schedule struct {
	ID         int    `json:"id" yaml:"id"`
	Name       string `json:"name" yaml:"name"`
	CronFormat string `json:"cron_format" yaml:"cron_format"`
	Active     bool   `json:"active" yaml:"active"`
	TemplateID int    `json:"template_id" yaml:"template_id"`
}

type AccessKey struct {
	ID   int    `json:"id" yaml:"id"`
	Name string `json:"name" yaml:"name"`
	Type string `json:"type" yaml:"type"`
}

type Repository struct {
	ID        int    `json:"id" yaml:"id"`
	Name      string `json:"name" yaml:"name"`
	GitURL    string `json:"git_url" yaml:"git_url"`
	GitBranch string `json:"git_branch" yaml:"git_branch"`
}

type View struct {
	ID       int    `json:"id" yaml:"id"`
	Title    string `json:"title" yaml:"title"`
	Position int    `json:"position" yaml:"position"`
}

type User struct {
	ID       int    `json:"id" yaml:"id"`
	Name     string `json:"name" yaml:"name"`
	Username string `json:"username" yaml:"username"`
	Email    string `json:"email" yaml:"email"`
	Admin    bool   `json:"admin" yaml:"admin"`
	External bool   `json:"external" yaml:"external"`
	Pro      bool   `json:"pro" yaml:"pro"`
	Created  string `json:"created" yaml:"created"`
}

type Runner struct {
	ID              int    `json:"id" yaml:"id"`
	Name            string `json:"name" yaml:"name"`
	Active          bool   `json:"active" yaml:"active"`
	MaxParallelTask int    `json:"max_parallel_tasks" yaml:"max_parallel_tasks"`
}

type Member struct {
	ID       int      `json:"id" yaml:"id"`
	Username string   `json:"username" yaml:"username"`
	Name     string   `json:"name" yaml:"name"`
	Role     string   `json:"role" yaml:"role"`
	Roles    []string `json:"roles" yaml:"roles"`
}

type APIToken struct {
	ID      string `json:"id" yaml:"id"`
	Name    string `json:"name" yaml:"name"`
	Created string `json:"created" yaml:"created"`
	Expired bool   `json:"expired" yaml:"expired"`
	UserID  int    `json:"user_id" yaml:"user_id"`
}
