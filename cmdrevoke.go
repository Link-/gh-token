package main

// Implementation of the "Revoke" command
type RevokeCmd struct {
	// ...
}

func (cmd *RevokeCmd) Run() error {
	// Build the logger and use it for any output
	logger := NewLogger(cli.Logging.Level, cli.Logging.Type)
	logger.Debug("RevokeCmd called")

	return nil
}
