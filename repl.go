package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mstarongithub/way2gay/repl"
	"github.com/mstarongithub/way2gay/util"
	"github.com/mstarongithub/way2gay/util/wrappers"
	"github.com/sirupsen/logrus"
)

func replRunner(server *Server) {
	// Give repl some wrappers around stdin and stdout so that it closes those instead of stdin & stdout themselves
	commandRepl := repl.NewRepl(wrappers.NewReaderWrapper(os.Stdin), wrappers.NewWriterWrapper(os.Stdout))
	logrus.Debugln("Starting repl")
	_ = commandRepl.Run(func(input string, r *repl.Repl) (string, error) {
		if cmdString, ok := strings.CutPrefix(input, "run "); ok {
			parts := strings.Split(cmdString, " ")
			// This is safe b/c it'll unpack into a slice of length 0
			args := parts[1:]
			// And here a slice of length 0 means that no additional arguments will be given
			// It's also safe if the repl command is "run " since the first element will now be an empty string
			// Which is also safe to "execute" since cmd.Start will just fail with the No Command error
			cmd := exec.Command(parts[0], args...)
			cmd.Stdout = r.Output
			cmd.Stderr = r.Output
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
			return "Running " + parts[0], nil
		} else if input == "quit" {
			server.Stop()
			time.Sleep(time.Second * 5)
			return "Quitting", errors.New("normal stop")
		} else if rawCmdString, ok := strings.CutPrefix(input, "inspect "); ok {
			// Can't unpack slices directly like in Python, so do it this roundabout way
			var target, mod, args string
			util.Unpack(strings.SplitN(rawCmdString, " ", 2), &target, &mod, &args)
			logrus.WithFields(logrus.Fields{
				"cmd":  target,
				"mod":  mod,
				"args": args,
				"raw":  rawCmdString,
			}).Debugln("Parsed inspect command")
			switch target {
			case "display":
				return "Display: No usable data", nil
			case "backend":
			case "renderer":
			case "allocator":
			case "scene":
			case "layout":
			case "xdgShell":
			case "topLevelList":
			case "cursor":
				switch mod {
				case "manager":
					return fmt.Sprintf("Cursor manager (no useful data): %+v", server.cursorMgr), nil
				case "mode":
					switch server.cursorMode {
					case CursorModeMove:
						return "Cursor mode: Move", nil
					case CursorModePassThrough:
						return "Cursor mode: PassThrough", nil
					case CursorModeResize:
						return "Cursor mode: Resize", nil
					default:
						return fmt.Sprintf("Cursor mode: Unknown: %+v", server.cursorMode), nil
					}
				default:
					return fmt.Sprintf(
							"Cursor: Location (%f:%f)",
							server.cursor.X(),
							server.cursor.Y()),
						nil
				}
			case "seat":
			case "keyboards":
			case "cursorMode":
			case "grabbedTopLevel":
			case "grabLocation":
			case "grabGeobox":
			case "edges":
			case "outputLayout":
			default:
				return "Placeholder", nil
			}
		} else {
			return "Unknown command", nil
		}
		return "Unknown command", nil
	})
}
