package main

import (
	"flag"
	"os"

	"github.com/golang/glog"
	"github.com/phonkee/goexpose"
)

func main() {
	configVar := flag.String("config", "config.json", "config file location")

	// Parse command line flags
	flag.Parse()

	var (
		config *goexpose.Config
		server *goexpose.Server
		err    error
	)

	// read config file
	if config, err = goexpose.NewConfigFromFilename(*configVar); err != nil {
		glog.Errorf("config error: %v", err)
		os.Exit(1)
	}

	// change working directory to config directory
	if err = os.Chdir(config.Directory); err != nil {
		glog.Errorf("config error: %v", err)
		os.Exit(1)
	}

	if server, err = goexpose.NewServer(config); err != nil {
		glog.Errorf("server error: %v", err)
		os.Exit(1)
	}

	if err = server.Run(); err != nil {
		glog.Errorf("server run error: %v", err)
		os.Exit(1)
	}

}
