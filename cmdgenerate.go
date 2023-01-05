package main

// Implementation of the "Generate" command
type GenerateCmd struct {
	// ...
}

func (cmd *GenerateCmd) Run() error {
	// Build the logger and use it for any output
	logger := NewLogger(cli.Logging.Level, cli.Logging.Type)
	logger.Debug("GenerateCmd called")

	return nil
}
