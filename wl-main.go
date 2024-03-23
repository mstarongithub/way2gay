package main

import (
	"fmt"
	"os"

	"github.com/mstarongithub/way2gay/config"
	"github.com/sirupsen/logrus"
	"github.com/swaywm/go-wlroots/wlroots"
)

func fatal(msg string, err error) {
	fmt.Printf("error %s: %s\n", msg, err)
	os.Exit(1)
}

func wlMain(conf *config.Config) {
	wlroots.OnLog(wlroots.LogImportanceError, func(importance wlroots.LogImportance, msg string) {
		switch importance {
		case wlroots.LogImportanceDebug:
			logrus.Debugln(msg)
		case wlroots.LogImportanceInfo:
			logrus.Infoln(msg)
		case wlroots.LogImportanceError:
			logrus.Errorln(msg)
		case wlroots.LogImportanceSilent:
			return
		}
	})

	// start the server
	server, err := NewServer()
	if err != nil {
		fatal("initializing server", err)
	}
	if err = server.Start(); err != nil {
		fatal("starting server", err)
	}

	go replRunner(server)

	// start the wayland event loop
	if err = server.Run(); err != nil {
		fatal("running server", err)
	}
}
