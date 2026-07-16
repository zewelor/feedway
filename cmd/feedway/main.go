package main

import (
	"errors"
	"os"

	"github.com/zewelor/feedway/internal/cli"
	"github.com/zewelor/feedway/internal/config"
)

func main() {
	os.Exit(cli.Run(
		os.Args[1:],
		os.LookupEnv,
		os.Stderr,
		func(config.Config) error {
			return errors.New("serve is not implemented")
		},
		func(config.Config) error {
			return errors.New("migrate is not implemented")
		},
	))
}
