package main

import (
	_ "embed"

	"alvus/internal/cmd"
)

//go:embed dashboard.html
var dashboardHTML string

func main() {
	cmd.Execute(dashboardHTML)
}