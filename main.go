package goexpose

import (
	_ "embed"
	"fmt"
	"github.com/phonkee/goexpose/internal/server"
	"github.com/urfave/cli/v2"
)

var (
	//go:embed VERSION
	version string
)

// NewApp returns cli app
// this is useful for those who wants to add additional functionality to goexpose (tasks, auth, etc..)
func NewApp() *cli.App {
	app := cli.NewApp()
	app.Authors = []*cli.Author{
		{
			Name:  "Peter Vrba",
			Email: "phonkee@phonkee.eu",
		},
	}
	app.Flags = []cli.Flag{
		&cli.StringFlag{Name: "config", Usage: "config filename"},
	}
	app.Usage = fmt.Sprintf(server.Logo, version)
	app.Version = version
	app.Commands = []*cli.Command{
		{
			Name:  "serve",
			Usage: "Starts goexpose server",
		},
		{
			Name:  "validate",
			Usage: "Validate config file",
		},
	}

	return app
}
