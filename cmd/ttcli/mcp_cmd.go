package main

import (
	"context"
	"fmt"
	"os"

	"github.com/j4y-w4lk3r/ttcli/internal/mcp"
)

func cmdMCP(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("usage: ttcli mcp")
	}
	c, err := client()
	if err != nil {
		return err
	}
	return mcp.Run(context.Background(), mcp.NewTickTickAPI(c), os.Stdin, os.Stdout)
}
