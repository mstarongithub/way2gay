package main

import (
	"flag"
	"runtime"

	"github.com/mstarongithub/way2gay/config"
	"github.com/sirupsen/logrus"
)

type StartMode int

// Start modes
const (
	// Start as compositor
	START_MODE_COMPOSITOR = StartMode(iota)
	// Utilise tools, no compositing
	START_MODE_TOOL
)

// command line flags
var (
	// Path to the config file. Default is "config.toml"
	configFile *string = flag.String("config", "config.toml", "path to the config file")
	asTool     *bool   = flag.Bool("tool", false, "start as a tool instead of a compositor")
)

func init() {
	// lock the main goroutine onto the current OS thread
	// we need to do this because EGL uses thread local storage
	// ----
	// So far not found what happens if this isn't done - Melody
	runtime.LockOSThread()
}

func main() {
	flag.Parse()

	// set up logging
	logrus.SetLevel(logrus.DebugLevel)
	logrus.WithFields(logrus.Fields{
		"configFile": *configFile,
		"asTool":     *asTool,
	}).Debugln("Flags")

	// Get config
	conf, err := config.ParseConfig(*configFile)
	if err != nil {
		logrus.WithError(err).Warnln("Using config fall-back")
	}

	// And now run depending on mode
	if *asTool {
		utilMain(conf)
	} else {
		wlMain(conf)
	}
}
