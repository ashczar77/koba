package main

import (
	"context"

	"koba/internal/cli"
)

func main() {
	cli.Run(context.Background(), false)
}
