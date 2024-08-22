package main

import (
	"flag"
	"fmt"

	"github.com/mstarongithub/way2gay/config"
	"github.com/sirupsen/logrus"
	"github.com/swaywm/go-wlroots/wlroots"
	"gitlab.com/mstarongitlab/goutils/sliceutils"
)

var (
	utilAction *string = flag.String(
		"action",
		"",
		"The action to perform. Can be one of:"+
			"\n\t- none: Do nothing"+
			"\n\t- outputs: List available outputs"+
			"\n\t- modes <output>: List available modes for an output",
	)
	outputSelection *string = flag.String(
		"output",
		"",
		"Output to perform the action on. Required for some actions",
	)
)

func utilMain(_ *config.Config) {
	if *help {
		utilHelpMessage()
		return
	}

	// Init a server, used for stuff like getting displays
	server, err := NewServer()
	if err != nil {
		logrus.WithError(err).Fatal("initializing server")
	}
	if err = server.Start(); err != nil {
		logrus.WithError(err).Fatal("starting server")
	}

	switch *utilAction {
	case "outputs":
		utilListOutputs(server)
	case "modes":
		if *outputSelection == "" {
			fmt.Println("Output has to be specified")
			return
		} else {
			utilListOutputModes(server, *outputSelection)
		}
	}
}

func utilHelpMessage() {
	fmt.Println("---- Help message for Way2Gay in tool mode ----")
	fmt.Println("\nIn tool mode, w2g will offer various tools for figuring out configurations and similar")
	fmt.Println("\nGeneral flags:")
	fmt.Println("\t-config: Path to the config file. Default is \"config.toml\"")
	fmt.Println("\t-tool: Start as a tool instead of a compositor")
	fmt.Println("\t-help: Show this help message (or the one for compositor mode if -tool is not set)")
	fmt.Println("\nTool flags:")
	fmt.Println("\t-action: The action to perform. Can be one of:")
	fmt.Println("\t\t- (default) outputs: List available outputs")
	fmt.Println("\t\t- modes: List available modes for an output. Use with -output")
	fmt.Println("\t-output: Output to perform the action on. Required for -action modes")
}

func utilListOutputs(server *Server) {
	outputs := server.GetOutputs()
	for i, output := range outputs {
		fmt.Printf("Output %v: %s\n", i, output.Name())
	}
}

func utilListOutputModes(server *Server, outputName string) {
	outputs := server.GetOutputs()
	filtered := sliceutils.Filter(outputs, func(output *wlroots.Output) bool {
		return output.Name() == outputName
	})
	if len(filtered) == 0 {
		fmt.Printf("Output %s not found\n", outputName)
		return
	}
	modes := filtered[0].Modes()
	fmt.Printf("Modes for output %s:\n", outputName)
	for _, mode := range modes {
		if mode.Preferred() {
			fmt.Printf("\t- %dx%d@%d(Ratio: %d) (preferred)\n", mode.Width(), mode.Height(), mode.Refresh(), mode.PictureAspectRatio())
		} else {
			fmt.Printf("\t- %dx%d@%d(Ratio: %d)\n", mode.Width(), mode.Height(), mode.Refresh(), mode.PictureAspectRatio())
		}
	}
}
