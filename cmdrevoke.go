package main

import "fmt"

// Implementation of the "Revoke" command
type RevokeCmd struct {
	// ...
}

func (cmd *RevokeCmd) Run(ctx *Context) error {
	// ...
	fmt.Println("Revoke")
	return nil
}
