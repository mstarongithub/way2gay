package main

import (
	"container/list"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/swaywm/go-wlroots/wlroots"
	"github.com/swaywm/go-wlroots/xkb"
)

type CursorMode int

const (
	CursorModePassThrough CursorMode = iota
	CursorModeMove
	CursorModeResize
)

type Server struct {
	display     wlroots.Display // TODO: Refactor into slice of displays
	backend     wlroots.Backend
	renderer    wlroots.Renderer
	allocator   wlroots.Allocator
	scene       wlroots.Scene
	sceneLayout wlroots.SceneOutputLayout

	xdgShell     wlroots.XDGShell
	topLevelList list.List

	cursor    wlroots.Cursor
	cursorMgr wlroots.XCursorManager

	seat            wlroots.Seat
	keyboards       []*Keyboard
	cursorMode      CursorMode
	grabbedTopLevel *wlroots.XDGTopLevel
	grabX, grabY    float64
	grabGeobox      wlroots.GeoBox
	resizeEdges     wlroots.Edges

	outputLayout wlroots.OutputLayout

	outputs []*wlroots.Output
}

type Keyboard struct {
	dev wlroots.InputDevice
}

func (server *Server) inTopLevel(topLevel *wlroots.XDGTopLevel) *list.Element {
	for e := server.topLevelList.Front(); e != nil; e = e.Next() {
		if *e.Value.(*wlroots.XDGTopLevel) == *topLevel {
			return e
		}
	}
	return nil
}

func (server *Server) moveFrontTopLevel(topLevel *wlroots.XDGTopLevel) {
	logrus.WithField("server.topLevelList.Len", server.topLevelList.Len()).Debugln("moveFrontTopLevel")
	e := server.inTopLevel(topLevel)
	if e != nil {
		logrus.WithField("topLevel", topLevel).Debugln("moveFrontTopLevel")
		server.topLevelList.MoveToFront(e)
	}
	logrus.WithField("server.topLevelList.Len", server.topLevelList.Len()).Debugln("moveFrontTopLevel")
}

func (server *Server) removeTopLevel(topLevel *wlroots.XDGTopLevel) {
	logrus.WithField("server.topLevelList.Len", server.topLevelList.Len()).Debugln("removeTopLevel")
	e := server.inTopLevel(topLevel)
	if e != nil {
		logrus.WithField("topLevel", topLevel).Debugln("removeTopLevel")
		server.topLevelList.Remove(e)
	}
	logrus.WithField("server.topLevelList.Len", server.topLevelList.Len()).Debugln("removeTopLevel")
}

func (server *Server) focusTopLevel(topLevel *wlroots.XDGTopLevel, surface *wlroots.Surface) {
	/* Note: this function only deals with keyboard focus. */
	if topLevel == nil {
		return
	}
	prevSurface := server.seat.KeyboardState().FocusedSurface()
	logrus.WithFields(logrus.Fields{
		"previous surface": prevSurface,
		"current surface":  *surface,
	}).Debugln("focusTopLevel")
	if prevSurface == *surface {
		/* Don't re-focus an already focused surface. */
		return
	}

	if !prevSurface.Nil() {
		/*
		 * Deactivate the previously focused surface. This lets the client know
		 * it no longer has focus and the client will repaint accordingly, e.g.
		 * stop displaying a caret.
		 */
		prevTopLevel, err := prevSurface.XDGTopLevel()
		if err == nil {
			prevTopLevel.SetActivated(false)
		}
	}

	/* Move the toplevel to the front */
	topLevel.Base().SceneTree().Node().RaiseToTop()
	logrus.WithFields(logrus.Fields{
		"server.topLevelList.Len": server.topLevelList.Len(),
		"topLevel":                topLevel,
	}).Debugln("focusTopLevel")
	server.moveFrontTopLevel(topLevel)
	logrus.WithField("server.topLevelList.Len", server.topLevelList.Len()).Debugln("focusTopLevel")
	/* Activate the new surface */
	topLevel.SetActivated(true)
	/*
	 * Tell the seat to have the keyboard enter this surface. wlroots will keep
	 * track of this and automatically send key events to the appropriate
	 * clients without additional work on your part.
	 */
	server.seat.NotifyKeyboardEnter(topLevel.Base().Surface(), server.seat.Keyboard())
}

func (server *Server) handleNewPointer(dev wlroots.InputDevice) {
	/* We don't do anything special with pointers. All of our pointer handling
	 * is proxied through wlr_cursor. On another compositor, you might take this
	 * opportunity to do libinput configuration on the device to set
	 * acceleration, etc. */
	server.cursor.AttachInputDevice(dev)
}

func (server *Server) handleKey(keyboard wlroots.Keyboard, time uint32, keyCode uint32, updateState bool, state wlroots.KeyState) {
	/* This event is raised when a key is pressed or released. */

	// translate libinput keycode to xkbcommon and obtain keysyms
	syms := keyboard.XKBState().Syms(xkb.KeyCode(keyCode + 8))

	handled := false
	modifiers := keyboard.Modifiers()
	if (modifiers&wlroots.KeyboardModifierAlt != 0) && state == wlroots.KeyStatePressed {
		/* If alt is held down and this button was _pressed_, we attempt to
		 * process it as a compositor keybinding. */
		for _, sym := range syms {
			handled = server.handleKeyBinding(sym)
		}
	}

	if !handled {
		/* Otherwise, we pass it along to the client. */
		server.seat.SetKeyboard(keyboard.Base())
		server.seat.NotifyKeyboardKey(time, keyCode, state)
	}
}

func (server *Server) handleNewKeyboard(dev wlroots.InputDevice) {
	keyboard := dev.Keyboard()

	/* We need to prepare an XKB keymap and assign it to the keyboard. This
	 * assumes the defaults (e.g. layout = "us"). */
	context := xkb.NewContext(xkb.KeySymFlagNoFlags)
	keymap := context.KeyMap()
	keyboard.SetKeymap(keymap)
	keymap.Destroy()
	context.Destroy()
	keyboard.SetRepeatInfo(25, 600)

	/* Here we set up listeners for keyboard events. */
	keyboard.OnModifiers(func(keyboard wlroots.Keyboard) {
		/* This event is raised when a modifier key, such as shift or alt, is
		* pressed. We simply communicate this to the client. */
		server.seat.SetKeyboard(dev)
		server.seat.NotifyKeyboardModifiers(keyboard)
	})
	keyboard.OnKey(server.handleKey)

	server.seat.SetKeyboard(dev)

	/* And add the keyboard to our list of keyboards */
	server.keyboards = append(server.keyboards, &Keyboard{dev: dev})
}

func (server *Server) handleNewInput(dev wlroots.InputDevice) {
	/* This event is raised by the backend when a new input device becomes
	 * available. */
	switch dev.Type() {
	case wlroots.InputDeviceTypePointer:
		server.handleNewPointer(dev)
	case wlroots.InputDeviceTypeKeyboard:
		server.handleNewKeyboard(dev)
	}

	/* We need to let the wlr_seat know what our capabilities are, which is
	 * communicated to the client. In TinyWL we always have a cursor, even if
	 * there are no pointer devices, so we always include that capability. */
	caps := wlroots.SeatCapabilityPointer
	if len(server.keyboards) > 0 {
		caps |= wlroots.SeatCapabilityKeyboard
	}
	server.seat.SetCapabilities(caps)
}

func (server *Server) topLevelAt(lx float64, ly float64) (*wlroots.XDGTopLevel, *wlroots.Surface, float64, float64) {
	/* This returns the topmost node in the scene at the given layout coords.
	 * We only care about surface nodes as we are specifically looking for a
	 * surface in the surface tree of a tinywl_toplevel. */

	node, sx, sy := server.scene.Tree().Node().At(lx, ly)

	if node.Nil() || node.Type() != wlroots.SceneNodeBuffer {
		return nil, nil, 0, 0
	}
	sceneSurface := node.SceneBuffer().SceneSurface()
	logrus.WithField("sceneSurface", sceneSurface).Debugln("topLevelAt")
	if sceneSurface.Nil() {
		return nil, nil, 0, 0
	}
	surface := sceneSurface.Surface()
	logrus.WithField("sceneSurface", sceneSurface).Debugln("topLevelAt")

	/* Find the node corresponding to the tinywl_toplevel at the root of this
	 * surface tree, it is the only one for which we set the data field. */

	topLevel := surface.XDGSurface().TopLevel()
	logrus.WithFields(logrus.Fields{
		"topLevel":             topLevel,
		"s.topLevelList.Len()": server.topLevelList.Len(),
	}).Debugln("topLevelAt")

	if server.inTopLevel(&topLevel) != nil {
		return &topLevel, &surface, sx, sy
	} else {
		return nil, &surface, sx, sy
	}
}

func (server *Server) handleNewFrame(output wlroots.Output) {
	/* This function is called every time an output is ready to display a frame,
	 * generally at the output's refresh rate (e.g. 60Hz). */

	logrus.WithField("name", output.Name()).Debugln("Output ready for frame")

	sOut, err := server.scene.SceneOutput(output)
	if err != nil {
		return
	}

	/* Render the scene if needed and commit the output */
	sOut.Commit()
	sOut.SendFrameDone(time.Now())
}

func (server *Server) handleOutputRequestState(output wlroots.Output, state wlroots.OutputState) {
	/* This function is called when the backend requests a new state for
	 * the output. For example, Wayland and X11 backends request a new mode
	 * when the output window is resized. */
	logrus.WithFields(logrus.Fields{
		"output": output,
		"state":  state,
	}).Debugln("New state request for output")
	output.CommitState(state)
}

func (server *Server) handleOuptuDestroy(output wlroots.Output) {
	logrus.WithField("name", output.Name()).Debugln("Output getting destroyed")
}

// TODO: Somehow add a method to get all available outputs
func (server *Server) handleNewOutput(output wlroots.Output) {
	/* This event is raised by the backend when a new output (aka a display or
	 * monitor) becomes available. */

	logrus.WithField("name", output.Name()).Debugln("New output added")
	server.outputs = append(server.outputs, &output)

	/* Configures the output created by the backend to use our allocator
	 * and our renderer. Must be done once, before commiting the output */
	output.InitRender(server.allocator, server.renderer)

	/* The output may be disabled, switch it on. */
	oState := wlroots.NewOutputState()
	oState.StateInit()
	oState.StateSetEnabled(true)

	/* Some backends don't have modes. DRM+KMS does, and we need to set a mode
	 * before we can use the output. The mode is a tuple of (width, height,
	 * refresh rate), and each monitor supports only a specific set of modes. We
	 * just pick the monitor's preferred mode, a more sophisticated compositor
	 * would let the user configure it. */
	mode, err := output.PrefferedMode()
	if err == nil {
		oState.SetMode(mode)
	}

	/* Atomically applies the new output state. */
	output.CommitState(oState)
	oState.Finish()

	/* Sets up a listener for the frame event. */
	output.OnFrame(server.handleNewFrame)

	/* Sets up a listener for the state request event. */
	output.OnRequestState(server.handleOutputRequestState)

	/* Sets up a listener for the destroy event. */
	output.OnDestroy(server.handleOuptuDestroy)

	/* Adds this to the output layout. The add_auto function arranges outputs
	 * from left-to-right in the order they appear. A more sophisticated
	 * compositor would let the user configure the arrangement of outputs in the
	 * layout.
	 *
	 * The output layout utility automatically adds a wl_output global to the
	 * display, which Wayland clients can see to find out information about the
	 * output (such as DPI, scale factor, manufacturer, etc).
	 */
	lOutput := server.outputLayout.AddOutputAuto(output)
	sceneOutput := server.scene.NewOutput(output)
	server.sceneLayout.AddOutput(lOutput, sceneOutput)

	err = output.SetTitle(fmt.Sprintf("tinywl (go-wlroots) - %s", output.Name()))
	if err != nil {
		return
	}
}

func (server *Server) handleCursorMotion(dev wlroots.InputDevice, time uint32, dx float64, dy float64) {
	/* This event is forwarded by the cursor when a pointer emits a _relative_
	 * pointer motion event (i.e. a delta) */

	/* The cursor doesn't move unless we tell it to. The cursor automatically
	 * handles constraining the motion to the output layout, as well as any
	 * special configuration applied for the specific input device which
	 * generated the event. You can pass NULL for the device if you want to move
	 * the cursor around without any input. */
	server.cursor.Move(dev, dx, dy)
	server.processCursorMotion(time)
}

func (server *Server) handleCursorMotionAbsolute(dev wlroots.InputDevice, time uint32, x float64, y float64) {
	/* This event is forwarded by the cursor when a pointer emits an _absolute_
	 * motion event, from 0..1 on each axis. This happens, for example, when
	 * wlroots is running under a Wayland window rather than KMS+DRM, and you
	 * move the mouse over the window. You could enter the window from any edge,
	 * so we have to warp the mouse there. There is also some hardware which
	 * emits these events. */
	server.cursor.WarpAbsolute(dev, x, y)
	server.processCursorMotion(time)
}

func (server *Server) processCursorMotion(time uint32) {
	/* If the mode is non-passthrough, delegate to those functions. */
	if server.cursorMode == CursorModeMove {
		server.processCursorMove(time)
		return
	} else if server.cursorMode == CursorModeResize {
		server.processCursorResize(time)
		return
	}

	/* Otherwise, find the toplevel under the pointer and send the event along. */
	topLevel, surface, sx, sy := server.topLevelAt(server.cursor.X(), server.cursor.Y())
	if topLevel == nil {
		/* If there's no toplevel under the cursor, set the cursor image to a
		 * default. This is what makes the cursor image appear when you move it
		 * around the screen, not over any toplevels. */
		server.cursor.SetXCursor(server.cursorMgr, "default")
	}
	if surface != nil {
		/*
		 * Send pointer enter and motion events.
		 *
		 * The enter event gives the surface "pointer focus", which is distinct
		 * from keyboard focus. You get pointer focus by moving the pointer over
		 * a window.
		 *
		 * Note that wlroots will avoid sending duplicate enter/motion events if
		 * the surface has already has pointer focus or if the client is already
		 * aware of the coordinates passed.
		 */
		server.seat.NotifyPointerEnter(*surface, sx, sy)
		server.seat.NotifyPointerMotion(time, sx, sy)
	} else {
		/* Clear pointer focus so future button events and such are not sent to
		 * the last client to have the cursor over it. */
		server.seat.ClearPointerFocus()
	}
}

func (server *Server) processCursorMove(_ uint32) {
	/* Move the grabbed toplevel to the new position. */
	server.grabbedTopLevel.Base().SceneTree().Node().SetPosition(server.cursor.X()-server.grabX, server.cursor.Y()-server.grabY)
}

func (server *Server) processCursorResize(_ uint32) {
	/*
	 * Resizing the grabbed toplevel can be a little bit complicated, because we
	 * could be resizing from any corner or edge. This not only resizes the
	 * toplevel on one or two axes, but can also move the toplevel if you resize
	 * from the top or left edges (or top-left corner).
	 *
	 * Note that some shortcuts are taken here. In a more fleshed-out
	 * compositor, you'd wait for the client to prepare a buffer at the new
	 * size, then commit any movement that was prepared.
	 */

	// borderX := s.cursor.X() - s.grabX
	// borderY := s.cursor.Y() - s.grabY
	borderX := server.cursor.X()
	borderY := server.cursor.Y()
	nLeft := server.grabGeobox.X
	nRight := server.grabGeobox.X + server.grabGeobox.Width
	nTop := server.grabGeobox.Y
	nBottom := server.grabGeobox.Y + server.grabGeobox.Height

	if server.resizeEdges&wlroots.EdgeTop != 0 {
		nTop = int(borderY)
		if nTop >= nBottom {
			nTop = nBottom - 1
		}
	} else if server.resizeEdges&wlroots.EdgeBottom != 0 {
		nBottom = int(borderY)
		if nBottom <= nTop {
			nBottom = nTop + 1
		}
	}

	if server.resizeEdges&wlroots.EdgeLeft != 0 {
		nLeft = int(borderX)
		if nLeft >= nRight {
			nLeft = nRight - 1
		}
	} else if server.resizeEdges&wlroots.EdgeRight != 0 {
		nRight = int(borderX)
		if nRight <= nLeft {
			nRight = nLeft + 1
		}
	}

	nWidth := nRight - nLeft
	nHeight := nBottom - nTop
	server.grabbedTopLevel.Base().TopLevelSetSize(uint32(nWidth), uint32(nHeight))
}

func (server *Server) handleSetCursorRequest(client wlroots.SeatClient, surface wlroots.Surface, _ uint32, hotspotX int32, hotspotY int32) {
	/* This event is raised by the seat when a client provides a cursor image */

	focusedClient := server.seat.PointerState().FocusedClient()

	/* This can be sent by any client, so we check to make sure this one is
	 * actually has pointer focus first. */
	if focusedClient == client {
		/* Once we've vetted the client, we can tell the cursor to use the
		 * provided surface as the cursor image. It will set the hardware cursor
		 * on the output that it's currently on and continue to do so as the
		 * cursor moves between outputs. */
		server.cursor.SetSurface(surface, hotspotX, hotspotY)
	}
}

func (server *Server) resetCursorMode() {
	/* Reset the cursor mode to passthrough. */
	server.cursorMode = CursorModePassThrough
	server.grabbedTopLevel = nil
}

func (server *Server) handleCursorButton(_ wlroots.InputDevice, time uint32, button uint32, state wlroots.ButtonState) {
	/* This event is forwarded by the cursor when a pointer emits a button
	 * event. */

	/* Notify the client with pointer focus that a button press has occurred */
	server.seat.NotifyPointerButton(time, button, state)

	if state == wlroots.ButtonStateReleased {
		/* If you released any buttons, we exit interactive move/resize mode. */
		server.resetCursorMode()
	} else {
		topLevel, surface, _, _ := server.topLevelAt(server.cursor.X(), server.cursor.Y())
		logrus.WithFields(logrus.Fields{
			"surface":  surface,
			"topLevel": topLevel,
		}).Debugln("handleCursorButton")
		/* Focus that client if the button was _pressed_ */
		server.focusTopLevel(topLevel, surface)
	}
}

func (server *Server) handleCursorAxis(_ wlroots.InputDevice, time uint32, source wlroots.AxisSource, orientation wlroots.AxisOrientation, delta float64, deltaDiscrete int32) {
	/* This event is forwarded by the cursor when a pointer emits an axis event,
	 * for example when you move the scroll wheel. */

	/* Notify the client with pointer focus of the axis event. */
	server.seat.NotifyPointerAxis(time, orientation, delta, deltaDiscrete, source)
}

func (server *Server) handleCursorFrame() {
	/* This event is forwarded by the cursor when a pointer emits an frame
	 * event. Frame events are sent after regular pointer events to group
	 * multiple events together. For instance, two axis events may happen at the
	 * same time, in which case a frame event won't be sent in between. */

	/* Notify the client with pointer focus of the frame event. */
	server.seat.NotifyPointerFrame()
}

func (server *Server) handleKeyBinding(sym xkb.KeySym) bool {
	/*
	 * Here we handle compositor keybindings. This is when the compositor is
	 * processing keys, rather than passing them on to the client for its own
	 * processing.
	 *
	 * This function assumes Alt is held down.
	 */
	switch sym {
	case xkb.KeySymEscape:
		server.display.Terminate()
	case xkb.KeySymF1:
		/* Cycle to the next toplevel */
		if server.topLevelList.Len() < 2 {
			break
		}
		// focus the next view
		nextView := server.topLevelList.Front().Next().Value.(*wlroots.XDGTopLevel)
		nextSurface := nextView.Base().Surface()
		server.focusTopLevel(nextView, &nextSurface)
	default:
		return false
	}
	return true
}

func (server *Server) handleMapXDGToplevel(xdgSurface wlroots.XDGSurface) {
	/* Called when the surface is mapped, or ready to display on-screen. */

	topLevel := xdgSurface.TopLevel()
	surface := xdgSurface.Surface()
	logrus.WithFields(logrus.Fields{
		"topLevel":                topLevel,
		"server.topLevelList.Len": server.topLevelList.Len(),
	}).Debugln("handleMapXDGToplevel")
	server.topLevelList.PushFront(&topLevel)
	logrus.WithField("server.topLevelList.Len", server.topLevelList.Len()).Debugln("handleMapXDGToplevel")
	server.focusTopLevel(&topLevel, &surface)
	logrus.WithField("server.topLevelList.Len", server.topLevelList.Len()).Debugln("handleMapXDGToplevel")
}

func (server *Server) handleUnMapXDGToplevel(xdgSurface wlroots.XDGSurface) {
	/* Called when the surface is unmapped, and should no longer be shown. */

	/* Reset the cursor mode if the grabbed toplevel was unmapped. */
	topLevel := xdgSurface.TopLevel()
	if server.grabbedTopLevel != nil && topLevel == *server.grabbedTopLevel {
		server.resetCursorMode()
	}
	server.removeTopLevel(&topLevel)
}
func (server *Server) handleNewXDGSurface(xdgSurface wlroots.XDGSurface) {
	/* This event is raised when wlr_xdg_shell receives a new xdg xdgSurface from a
	 * client, either a toplevel (application window) or popup. */

	logrus.WithField("surface", xdgSurface).Debugln("New surface inbound")

	if xdgSurface.Role() == wlroots.XDGSurfaceRolePopup {

		parent := xdgSurface.Popup().Parent()
		if parent.Nil() {
			logrus.WithField("surface", xdgSurface).Fatalln("xdgSurface popup parent is nil")
		}
		xdgSurface.SetData(parent.XDGSurface().SceneTree().NewXDGSurface(xdgSurface))
		return
	}
	if xdgSurface.Role() != wlroots.XDGSurfaceRoleTopLevel {
		logrus.WithFields(logrus.Fields{
			"surface": xdgSurface,
			"role":    xdgSurface.Role(),
		}).Fatalln("xdgSurface role is not XDGSurfaceRoleTopLevel")
	}

	xdgSurface.SetData(server.scene.Tree().NewXDGSurface(xdgSurface.TopLevel().Base()))
	xdgSurface.OnMap(server.handleMapXDGToplevel)
	xdgSurface.OnUnmap(server.handleUnMapXDGToplevel)
	xdgSurface.OnDestroy(func(surface wlroots.XDGSurface) {})

	toplevel := xdgSurface.TopLevel()
	toplevel.OnRequestMove(func(client wlroots.SeatClient, serial uint32) {
		server.beginInteractive(&toplevel, CursorModeMove, 0)
	})
	toplevel.OnRequestResize(func(client wlroots.SeatClient, serial uint32, edges wlroots.Edges) {
		server.beginInteractive(&toplevel, CursorModeResize, edges)
	})
}

func (server *Server) beginInteractive(topLevel *wlroots.XDGTopLevel, mode CursorMode, edges wlroots.Edges) {
	/* This function sets up an interactive move or resize operation, where the
	 * compositor stops propegating pointer events to clients and instead
	 * consumes them itself, to move or resize windows. */
	if topLevel.Base().Surface() != server.seat.PointerState().FocusedSurface() {
		/* Deny move/resize requests from unfocused clients. */
		return
	}
	server.grabbedTopLevel = topLevel
	server.cursorMode = mode

	if mode == CursorModeMove {
		server.grabX = server.cursor.X() - float64(topLevel.Base().SceneTree().Node().X())
		server.grabY = server.cursor.Y() - float64(topLevel.Base().SceneTree().Node().Y())
	} else {
		box := topLevel.Base().Geometry()
		r := func() int {
			if edges&wlroots.EdgeRight != 0 {
				return box.Width
			} else {
				return 0
			}
		}()
		b := func() int {
			if edges&wlroots.EdgeBottom != 0 {
				return box.Height
			} else {
				return 0
			}
		}()
		borderX := (topLevel.Base().SceneTree().Node().X() + box.X) + r
		borderY := (topLevel.Base().SceneTree().Node().Y() + box.Y) + b
		server.grabX = server.cursor.X() + float64(borderX)
		server.grabY = server.cursor.Y() + float64(borderY)
		server.grabGeobox = box
		server.grabGeobox.X += topLevel.Base().SceneTree().Node().X()
		server.grabGeobox.Y += topLevel.Base().SceneTree().Node().Y()

		server.resizeEdges = edges
	}
}

func (server *Server) GetOutputs() []*wlroots.Output {
	return server.outputs
}

func NewServer() (server *Server, err error) {
	server = new(Server)

	/* The Wayland display is managed by libwayland. It handles accepting
	 * clients from the Unix socket, manging Wayland globals, and so on. */
	server.display = wlroots.NewDisplay()

	/* The backend is a wlroots feature which abstracts the underlying input and
	 * output hardware. The autocreate option will choose the most suitable
	 * backend based on the current environment, such as opening an X11 window
	 * if an X11 server is running. */
	server.backend, err = server.display.BackendAutocreate()
	if err != nil {
		return nil, err
	}

	/* Autocreates a renderer, either Pixman, GLES2 or Vulkan for us. The user
	 * can also specify a renderer using the WLR_RENDERER env var.
	 * The renderer is responsible for defining the various pixel formats it
	 * supports for shared memory, this configures that for clients. */
	server.renderer, err = server.backend.RendererAutoCreate()
	if err != nil {
		return nil, err
	}
	server.renderer.InitDisplay(server.display)

	/* Autocreates an allocator for us.
	 * The allocator is the bridge between the renderer and the backend. It
	 * handles the buffer creation, allowing wlroots to render onto the
	 * screen */
	server.allocator, err = server.backend.AllocatorAutocreate(server.renderer)
	if err != nil {
		return nil, err
	}

	/* This creates some hands-off wlroots interfaces. The compositor is
	 * necessary for clients to allocate surfaces, the subcompositor allows to
	 * assign the role of subsurfaces to surfaces and the data device manager
	 * handles the clipboard. Each of these wlroots interfaces has room for you
	 * to dig your fingers in and play with their behavior if you want. Note that
	 * the clients cannot set the selection directly without compositor approval,
	 * see the handling of the request_set_selection event below.*/
	server.display.CompositorCreate(5, server.renderer)
	server.display.SubCompositorCreate()
	server.display.DataDeviceManagerCreate()

	/* Creates an output layout, which a wlroots utility for working with an
	 * arrangement of screens in a physical layout. */
	server.outputLayout = wlroots.NewOutputLayout()

	/* Configure a listener to be notified when new outputs are available on the
	 * backend. */
	server.backend.OnNewOutput(server.handleNewOutput)

	/* Create a scene graph. This is a wlroots abstraction that handles all
	 * rendering and damage tracking. All the compositor author needs to do
	 * is add things that should be rendered to the scene graph at the proper
	 * positions and then call wlr_scene_output_commit() to render a frame if
	 * necessary.
	 */
	server.scene = wlroots.NewScene()
	server.sceneLayout = server.scene.AttachOutputLayout(server.outputLayout)

	/* Set up xdg-shell version 3. The xdg-shell is a Wayland protocol which is
	 * used for application windows. For more detail on shells, refer to
	 * https://drewdevault.com/2018/07/29/Wayland-shells.html.
	 */
	server.topLevelList.Init()
	server.xdgShell = server.display.XDGShellCreate(3)
	server.xdgShell.OnNewSurface(server.handleNewXDGSurface)

	/*
	 * Creates a cursor, which is a wlroots utility for tracking the cursor
	 * image shown on screen.
	 */
	server.cursor = wlroots.NewCursor()
	server.cursor.AttachOutputLayout(server.outputLayout)

	/* Creates an xcursor manager, another wlroots utility which loads up
	 * Xcursor themes to source cursor images from and makes sure that cursor
	 * images are available at all scale factors on the screen (necessary for
	 * HiDPI support). */
	server.cursorMgr = wlroots.NewXCursorManager("", 24)

	/*
	 * wlr_cursor *only* displays an image on screen. It does not move around
	 * when the pointer moves. However, we can attach input devices to it, and
	 * it will generate aggregate events for all of them. In these events, we
	 * can choose how we want to process them, forwarding them to clients and
	 * moving the cursor around. More detail on this process is described in
	 * https://drewdevault.com/2018/07/17/Input-handling-in-wlroots.html.
	 *
	 * And more comments are sprinkled throughout the notify functions above.
	 */
	server.cursorMode = CursorModePassThrough
	server.cursor.OnMotion(server.handleCursorMotion)
	server.cursor.OnMotionAbsolute(server.handleCursorMotionAbsolute)
	server.cursor.OnButton(server.handleCursorButton)
	server.cursor.OnAxis(server.handleCursorAxis)
	server.cursor.OnFrame(server.handleCursorFrame)
	server.cursorMgr.Load(1)

	/*
	 * Configures a seat, which is a single "seat" at which a user sits and
	 * operates the computer. This conceptually includes up to one keyboard,
	 * pointer, touch, and drawing tablet device. We also rig up a listener to
	 * let us know when new input devices are available on the backend.
	 */
	server.backend.OnNewInput(server.handleNewInput)
	server.seat = server.display.SeatCreate("seat0")
	server.seat.OnSetCursorRequest(server.handleSetCursorRequest)

	return
}

func (server *Server) Start() error {

	/* Add a Unix socket to the Wayland display. */
	socket, err := server.display.AddSocketAuto()
	if err != nil {
		server.backend.Destroy()
		return err
	}
	logrus.WithField("socket", socket).Debugln("got wl socket")
	/* Start the backend. This will enumerate outputs and inputs, become the DRM
	 * master, etc */
	if err = server.backend.Start(); err != nil {
		server.backend.Destroy()
		server.display.Destroy()
		return err
	}

	/* Set the WAYLAND_DISPLAY environment variable to our socket and run the
	 * startup command if requested. */
	if res := os.Getenv("WAYLAND_DISPLAY"); res != "" {
		logrus.WithField("WAYLAND_DISPLAY", res).Debugln("Wayland display already set, overwriting")
	}
	if err = os.Setenv("WAYLAND_DISPLAY", socket); err != nil {
		return err
	}

	logrus.WithField("WAYLAND_DISPLAY", socket).Infoln("Running Wayland compositor")
	return err
}

func (server *Server) Run() error {

	/* Run the Wayland event loop. This does not return until you exit the
	 * compositor. Starting the backend rigged up all of the necessary event
	 * loop configuration to listen to libinput events, DRM events, generate
	 * frame events at the refresh rate, and so on. */
	server.display.Run()

	/* Once s.display.Run() returns, we destroy all clients then shut down the
	 * server. */
	server.display.DestroyClients()
	server.scene.Tree().Node().Destroy()
	server.cursorMgr.Destroy()
	server.outputLayout.Destroy()
	server.display.Destroy()
	return nil
}

func (server *Server) Stop() {
	server.display.Terminate()
}
