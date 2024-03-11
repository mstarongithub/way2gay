package main

import (
	"errors"
	"flag"
	"fmt"
	"strings"
	"time"

	"os"
	"os/exec"
	"runtime"

	"github.com/mstarongithub/way2gay/repl"
	"github.com/sirupsen/logrus"
	"github.com/swaywm/go-wlroots/wlroots"
)

var (
	command = flag.String("s", "", "startup command")
)

func fatal(msg string, err error) {
	fmt.Printf("error %s: %s\n", msg, err)
	os.Exit(1)
}

func init() {
	// lock the main goroutine onto the current OS thread
	// we need to do this because EGL uses thread local storage
	runtime.LockOSThread()
}

func main() {

	flag.Parse()

	// set up logging
	// programLevel.Set(slog.LevelDebug)
	wlroots.OnLog(wlroots.LogImportanceDebug, func(importance wlroots.LogImportance, msg string) {
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

	commandRepl := repl.NewRepl(nil, nil)
	go func() {
		_ = commandRepl.Run(func(input string, r *repl.Repl) (string, error) {
			if cmdString, ok := strings.CutPrefix(input, "run "); ok {
				parts := strings.Split(cmdString, " ")
				args := parts[1:]
				cmd := exec.Command(parts[0], args...)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				go func(cmd *exec.Cmd, cmdString string) {
					err := cmd.Start()
					if err != nil {
						logrus.WithError(err).WithField("command", cmdString).Errorln("Command failed to start")
						return
					}
					err = cmd.Wait()
					if exiterr, ok := err.(*exec.ExitError); ok {
						logrus.WithError(err).WithFields(logrus.Fields{
							"exit-code": exiterr.ExitCode(),
							"comand":    cmdString,
						}).Warningln("Bad command completion")
					}
				}(cmd, cmdString)
				return "", nil
			} else if input == "quit" {
				server.Stop()
				time.Sleep(time.Second * 5)
				return "Quitting", errors.New("normal stop")
			}
			return "Unknown command", nil
		})
	}()

	// start the wayland event loop
	if err = server.Run(); err != nil {
		fatal("running server", err)
	}
}
