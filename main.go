package main

import (
	"github.com/alecthomas/kong"
)

// Build the command line interface
// Each of the commands listed below have a corresponding cmd*.go file. e.g. cmdinstallations.go for Installations.
var cli struct {
	Logging struct {
		Level string `enum:"debug,info,warn,error" default:"info"`
		Type  string `enum:"json,console" default:"console"`
	} `embed:"" prefix:"logging."`

	GithubURL string `help:"Github API URL" default:"https://api.github.com" env:"GHTOKEN_GITHUB_URL"`

	Installations InstallationsCmd `cmd:"" help:"Find our Github app installations"`
	Generate      GenerateCmd      `cmd:"" help:"Generate a new Github app token"`
	Revoke        RevokeCmd        `cmd:"" help:"Revoke a Github app token"`
}

func main() {

	// Validate the CLI structure and pass down the logger
	ctx := kong.Parse(&cli,
		kong.Name("gh-token"),
		kong.Description("Generate an access token to call GitHub APIs using a GitHub App."),
		kong.UsageOnError(),
	)

	// Call the Run() method of the selected parsed command.
	err := ctx.Run(kong.Bind(cli.Logging))
	ctx.FatalIfErrorf(err)
}
