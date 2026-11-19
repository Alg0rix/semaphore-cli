package main

import (
	"embed"

	"semaphore-cli/cmd"
)

//go:embed skills
var skillsFS embed.FS

func main() {
	cmd.SkillsFS = skillsFS
	cmd.Execute()
}
