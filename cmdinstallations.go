package main

import "fmt"

// Implementation of the "installations" command
type InstallationsCmd struct {
	// Flags
	KeyFile string `help:"Path to the private key file (pem)." type:"existingfile" aliases:"key"`
	KeyBase string `help:"Base64 encoded private key." type:"string" aliases:"base64_key"`

	// Arguments
	AppID int `arg:"" help:"Github App ID." type:"int" aliases:"app_id" required:"true"`
}

func (cmd *InstallationsCmd) Run(ctx *Context) error {
	// ...
	fmt.Println("installations")
	return nil
}
