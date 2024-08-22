package main

import (
	"fmt"

	"github.com/mstarongithub/way2gay/config"
	"github.com/sirupsen/logrus"
	"github.com/swaywm/go-wlroots/wlroots"
)

func wlMain(_ *config.Config) {
	if *help {
		fmt.Println("---- Help message for Way2Gay in compositor mode ----")
		fmt.Println("\nCompositor mode is when w2g will start as a compositor")
		fmt.Println("\nGeneral flags:")
		fmt.Println("\t-config: Path to the config file. Default is \"config.toml\"")
		fmt.Println("\t-tool: Start as a tool instead of a compositor")
		fmt.Println("\t-help: Show this help message (or the one for tool mode if -tool is set)")
		return
	}

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
		logrus.WithError(err).Fatal("initializing server")
	}
	if err = server.Start(); err != nil {
		logrus.WithError(err).Fatal("starting server")
	}

	go replRunner(server)

	// start the wayland event loop
	if err = server.Run(); err != nil {
		logrus.WithError(err).Fatal("running server")
	}
}
