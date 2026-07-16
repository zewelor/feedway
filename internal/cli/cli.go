package cli

import (
	"fmt"
	"io"

	"github.com/zewelor/feedway/internal/config"
)

const usage = "Usage: feedway <serve|migrate>\n"

type Command func(config.Config) error

func Run(
	args []string,
	lookupEnv config.LookupEnv,
	stderr io.Writer,
	serve Command,
	migrate Command,
) int {
	if len(args) != 1 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	var command Command
	switch args[0] {
	case "serve":
		command = serve
	case "migrate":
		command = migrate
	default:
		fmt.Fprint(stderr, usage)
		return 2
	}

	configuration, err := config.Load(lookupEnv)
	if err != nil {
		fmt.Fprintf(stderr, "feedway: %v\n", err)
		return 1
	}
	if err := command(configuration); err != nil {
		fmt.Fprintf(stderr, "feedway: %v\n", err)
		return 1
	}

	return 0
}
