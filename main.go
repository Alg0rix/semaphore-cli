package main

import (
	"embed"

	"github.com/Alg0rix/semaphore-cli/cmd"
)

//go:embed skills
var skillsFS embed.FS

func main() {
	cmd.SkillsFS = skillsFS
	cmd.Execute()
}
