package main

import (
	"github.com/alecthomas/kong"
)

type Context struct {
	Debug bool
}

// Build the command line interface
// Each of the commands listed below have a corresponding cmd*.go file. e.g. cmdinstallations.go for Installations.
var cli struct {
	Debug bool `help:"Enable debug mode."`

	Installations InstallationsCmd `cmd:"" help:"Find our Github app installations"`
	Generate      GenerateCmd      `cmd:"" help:"Generate a new Github app token"`
	Revoke        RevokeCmd        `cmd:"" help:"Revoke a Github app token"`
}

func main() {
	ctx := kong.Parse(&cli)
	// Call the Run() method of the selected parsed command.
	err := ctx.Run(&Context{Debug: cli.Debug})
	ctx.FatalIfErrorf(err)
}
