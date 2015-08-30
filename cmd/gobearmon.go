package main

import "github.com/lunanode/gobearmon"

import "os"

func main() {
	cfgPath := "gobearmon.cfg"
	if len(os.Args) >= 2 {
		cfgPath = os.Args[1]
	}
	gobearmon.Launch(cfgPath)
}
