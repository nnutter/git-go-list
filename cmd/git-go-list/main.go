package main

import (
	"context"
	"os"

	fang "charm.land/fang/v2"

	"github.com/nnutter/git-go-list/internal/cli"
)

func main() {
	if err := fang.Execute(
		context.Background(),
		cli.NewRoot(),
	); err != nil {
		os.Exit(1)
	}
}
