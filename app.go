package goexpose

import (
	"context"
	_ "embed"
	"fmt"
	"github.com/phonkee/goexpose/internal/config"
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
		&cli.StringFlag{
			Name:    "config",
			Usage:   "config filename accepts supported formats (json, yaml)",
			EnvVars: []string{"GOEXPOSE_CONFIG"},
			Aliases: []string{"c"},
		},
	}
	app.Usage = fmt.Sprintf(server.Logo, version)
	app.Version = version
	app.Commands = []*cli.Command{
		{
			Name:  "serve",
			Usage: "serve goexpose server",
			Action: func(c *cli.Context) error {
				cfg, err := config.NewFromFilename(c.String("config"))
				if err != nil {
					return err
				}

				srv, err := server.New(cfg)
				if err != nil {
					return err
				}

				return srv.Run(context.Background())
			},
		},
		{
			Name:  "validate",
			Usage: "validate config file",
			Action: func(c *cli.Context) error {
				return fmt.Errorf("not implemented")
			},
		},
	}

	return app
}
