package main

import (
	"embed"
	"log"
	"os"

	"github.com/wensboy/automan/backend/api/cli"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed data
var data embed.FS

func main() {
	if err := cli.Main(os.Args[1:], assets, data); err != nil {
		log.Fatal(err)
	}
}
