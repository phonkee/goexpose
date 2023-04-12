/*
Main package for goexpose binary.
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
}
