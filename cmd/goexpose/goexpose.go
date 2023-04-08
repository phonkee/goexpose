/*
Main package for goexpose binary.

Goexpose provides several command line arguments such as:
* config - configuration file
* format - format of configuration file (json, yaml), default is json
*/
package main

import (
	"github.com/phonkee/goexpose"
	"os"
)

func main() {
	app := goexpose.NewApp()

	if err := app.Run(os.Args); err != nil {
		panic(err)
	}

	//configVar := flag.String("cfg", "cfg.json", "Configuration file location")
	//formatVar := flag.String("format", "json", "Configuration file format. (json, yaml)")
	//
	//// Parse command line flags
	//flag.Parse()
	//
	//var (
	//	cfg *config.Config
	//	srv *server.Server
	//	err error
	//)
	//
	//// read cfg file
	//if cfg, err = config.NewConfigFromFilename(*configVar, *formatVar); err != nil {
	//	os.Exit(1)
	//}
	//
	//// change working directory to cfg directory
	//if err = os.Chdir(cfg.Directory); err != nil {
	//	os.Exit(1)
	//}
	//
	//if srv, err = server.New(cfg); err != nil {
	//	os.Exit(1)
	//}
	//
	//if err = srv.Run(context.Background()); err != nil {
	//	os.Exit(1)
	//}
}
