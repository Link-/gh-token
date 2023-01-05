package main

// Implementation of the "installations" command
type InstallationsCmd struct {

	// Arguments
	AppID   int    `arg:"" help:"Github App ID." type:"int" aliases:"app_id" env:"GHTOKEN_APP_ID" required:"true"`
	KeyFile string `arg:"" help:"Path to the private key file (pem)." type:"existingfile" aliases:"key" env:"GHTOKEN_KEY_FILE" required:"true"`
}

func (cmd *InstallationsCmd) Run() error {
	// Build the logger and use it for any output
	logger := NewLogger(cli.Logging.Level, cli.Logging.Type)
	logger.Debugf("InstallationsCmd called with AppID %v, KeyFile %v", cmd.AppID, cmd.KeyFile)

	return nil
}
