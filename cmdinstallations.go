package main

import "fmt"

// Implementation of the "installations" command
type InstallationsCmd struct {
	// ...
}

func (cmd *InstallationsCmd) Run(ctx *Context) error {
	// ...
	fmt.Println("installations")
	return nil
}
