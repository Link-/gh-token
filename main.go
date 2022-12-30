package main

import (
	"github.com/alecthomas/kong"
	"github.com/sirupsen/logrus"
)

type debugFlag bool

type Context struct {
}

func (d debugFlag) BeforeApply(logger *logrus.Logger) error {
	logger.SetLevel(logrus.DebugLevel)
	logger.Debugf("loglevel: %v", logger.GetLevel())
	return nil
}

// Build the command line interface
// Each of the commands listed below have a corresponding cmd*.go file. e.g. cmdinstallations.go for Installations.
var cli struct {
	Debug debugFlag `help:"Enable debug mode."`

	Installations InstallationsCmd `cmd:"" help:"Find our Github app installations"`
	Generate      GenerateCmd      `cmd:"" help:"Generate a new Github app token"`
	Revoke        RevokeCmd        `cmd:"" help:"Revoke a Github app token"`
}

func main() {

	// Create a new logger, this is passed down to each command
	var logger = logrus.New()

	// Validate the CLI structure and pass down the logger
	ctx := kong.Parse(&cli, kong.Bind(logger))

	// Call the Run() method of the selected parsed command.
	err := ctx.Run(&Context{})
	ctx.FatalIfErrorf(err)
}
