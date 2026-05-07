package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/aiwaki/makc"
)

func main() {
	var backendName string
	var keyboardBackendName string
	var inputTagName string
	var listenBackendName string
	var buttonName string
	var tapKey string
	var comboKeys string
	var typeText string
	var scanCode uint
	var scanExtended bool
	var listen bool
	var capabilities bool
	var listenCount int
	var includeInjected bool
	var normalizeOwnInjected bool
	var inject bool
	var click bool
	var absolute bool
	var wheel int
	var hwheel int
	var drag bool
	var profileName string
	var steps int
	var jitter int
	var seed int64
	var duration time.Duration
	var dx int
	var dy int
	var wait time.Duration

	flag.StringVar(&backendName, "backend", "auto", "mouse injection backend: auto, sendinput, injectmouseinput, cgevent, uinput")
	flag.StringVar(&keyboardBackendName, "keyboard-backend", "auto", "keyboard injection backend: auto, sendinput, injectkeyboardinput, cgevent, uinput")
	flag.StringVar(&inputTagName, "input-tag", "", "backend tag for injected inputs where supported; empty uses the per-client default, 0 disables tagging")
	flag.StringVar(&listenBackendName, "listen-backend", "auto", "listener backend: auto, hook, rawinput, evdev")
	flag.StringVar(&buttonName, "button", "left", "mouse button: left, right, middle, x1, x2")
	flag.StringVar(&tapKey, "tap", "", "keyboard key to tap")
	flag.StringVar(&comboKeys, "combo", "", "keyboard combo such as control+a")
	flag.StringVar(&typeText, "type", "", "Unicode text to type")
	flag.UintVar(&scanCode, "scan", 0, "scan code to tap")
	flag.BoolVar(&scanExtended, "scan-extended", false, "mark -scan as an extended key")
	flag.BoolVar(&listen, "listen", false, "listen for low-level mouse and keyboard events")
	flag.BoolVar(&capabilities, "capabilities", false, "probe backend capabilities without visible clicks or text input")
	flag.IntVar(&listenCount, "listen-count", 4, "number of events to print before stopping")
	flag.BoolVar(&includeInjected, "include-injected", false, "include injected events in listener output")
	flag.BoolVar(&normalizeOwnInjected, "normalize-own-injected", false, "clear injected markers in makc output for events tagged by this client")
	flag.BoolVar(&inject, "inject", false, "inject a small relative mouse move")
	flag.BoolVar(&absolute, "absolute", false, "treat dx and dy as absolute coordinates instead of a relative delta")
	flag.BoolVar(&click, "click", false, "click the left mouse button; requires -inject")
	flag.IntVar(&wheel, "wheel", 0, "vertical wheel detents to inject")
	flag.IntVar(&hwheel, "hwheel", 0, "horizontal wheel detents to inject")
	flag.BoolVar(&drag, "drag", false, "drag from the current position by dx,dy")
	flag.StringVar(&profileName, "profile", "ease", "movement profile used with -drag: instant, linear, ease, natural")
	flag.IntVar(&steps, "steps", 8, "movement profile steps used with -drag")
	flag.IntVar(&jitter, "jitter", 0, "natural movement max path jitter in pixels; 0 chooses a distance-based value")
	flag.Int64Var(&seed, "seed", 1, "natural movement seed used with -profile natural")
	flag.DurationVar(&duration, "duration", 120*time.Millisecond, "movement profile duration used with -drag")
	flag.IntVar(&dx, "dx", 1, "relative X movement used with -inject")
	flag.IntVar(&dy, "dy", 1, "relative Y movement used with -inject")
	flag.DurationVar(&wait, "wait", 100*time.Millisecond, "delay before reading position after injection")
	flag.Parse()

	backend, err := parseBackend(backendName)
	if err != nil {
		log.Fatal(err)
	}
	profile, err := parseMovementProfile(profileName, steps, duration, jitter, seed)
	if err != nil {
		log.Fatal(err)
	}
	keyboardBackend, err := parseKeyboardBackend(keyboardBackendName)
	if err != nil {
		log.Fatal(err)
	}
	button, err := parseButton(buttonName)
	if err != nil {
		log.Fatal(err)
	}
	inputTag, hasInputTag, err := parseInputTag(inputTagName)
	if err != nil {
		log.Fatal(err)
	}
	listenBackend, err := parseListenBackend(listenBackendName)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	opts := []makc.Option{
		makc.WithMouseInjection(backend),
		makc.WithKeyboardInjection(keyboardBackend),
	}
	if hasInputTag {
		opts = append(opts, makc.WithInputTag(inputTag))
	}
	client, err := makc.Open(opts...)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	fmt.Printf("runtime=%s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("mouse_injection=%s\n", client.Mouse.InjectionBackend())
	fmt.Printf("keyboard_injection=%s\n", client.Keyboard.InjectionBackend())
	fmt.Printf("input_tag=0x%X\n", client.InputTag())

	pos, hasPos := readPoint("position", func() (makc.Point, error) {
		return client.Mouse.Position(ctx)
	})
	readPoint("screen", func() (makc.Point, error) {
		return client.Mouse.ScreenSize(ctx)
	})
	readBool("left_button_down", func() (bool, error) {
		return client.Mouse.Down(ctx, makc.ButtonLeft)
	})
	readBool("key_a_down", func() (bool, error) {
		return client.Keyboard.Down(ctx, makc.KeyA)
	})

	if capabilities {
		if err := runCapabilitySmoke(ctx, client, listenBackend, pos, hasPos, includeInjected, normalizeOwnInjected); err != nil {
			log.Fatal(err)
		}
		return
	}

	if listen {
		if err := runListenSmoke(ctx, client, listenBackend, listenCount, includeInjected, normalizeOwnInjected); err != nil {
			log.Fatal(err)
		}
		return
	}

	if tapKey != "" {
		key, err := parseKey(tapKey)
		if err != nil {
			log.Fatal(err)
		}
		if err := client.Keyboard.Tap(ctx, key); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("tapped=%s\n", key)
	}
	if comboKeys != "" {
		keys, err := parseKeys(comboKeys)
		if err != nil {
			log.Fatal(err)
		}
		if err := client.Keyboard.Combo(ctx, keys...); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("combo=%s\n", comboKeys)
	}
	if scanCode != 0 {
		if err := client.Keyboard.ScanTap(ctx, uint16(scanCode), scanExtended); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("scan=0x%X extended=%v\n", scanCode, scanExtended)
	}
	if typeText != "" {
		if err := client.Keyboard.TypeText(ctx, typeText); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("typed_runes=%d\n", len([]rune(typeText)))
	}

	if wheel != 0 {
		if err := client.Mouse.Wheel(ctx, wheel); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("wheel=%d\n", wheel)
	}
	if hwheel != 0 {
		if err := client.Mouse.HWheel(ctx, hwheel); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("hwheel=%d\n", hwheel)
	}
	if drag {
		if !hasPos {
			log.Fatal("drag requires readable cursor position")
		}
		before := pos
		if err := client.Mouse.DragBy(ctx, button, dx, dy, profile); err != nil {
			log.Fatal(err)
		}
		time.Sleep(wait)
		pos, err = client.Mouse.Position(ctx)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("dragged=%d,%d profile=%s steps=%d duration=%s\n", dx, dy, profileName, steps, duration)
		fmt.Printf("position_after=%d,%d\n", pos.X, pos.Y)
		fmt.Printf("drag_verified=%v\n", pos != before)
		return
	}

	if !inject {
		return
	}

	move := makc.Rel(dx, dy)
	if absolute {
		move = makc.Abs(dx, dy)
	}
	if err := client.Mouse.Move(ctx, move); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("moved=%d,%d absolute=%v\n", dx, dy, absolute)
	time.Sleep(wait)

	posAfter, hasPosAfter := readPoint("position_after", func() (makc.Point, error) {
		return client.Mouse.Position(ctx)
	})
	if hasPos && hasPosAfter {
		fmt.Printf("move_verified=%v\n", posAfter != pos)
	}

	if click {
		if err := client.Mouse.Click(ctx, button); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("clicked=%s\n", button)
	}
}

func readPoint(name string, read func() (makc.Point, error)) (makc.Point, bool) {
	point, err := read()
	if err != nil {
		fmt.Printf("%s_error=%v\n", name, err)
		return makc.Point{}, false
	}
	fmt.Printf("%s=%d,%d\n", name, point.X, point.Y)
	return point, true
}

func readBool(name string, read func() (bool, error)) bool {
	value, err := read()
	if err != nil {
		fmt.Printf("%s_error=%v\n", name, err)
		return false
	}
	fmt.Printf("%s=%v\n", name, value)
	return true
}

func runCapabilitySmoke(
	ctx context.Context,
	client *makc.Client,
	listenBackend makc.ListenBackend,
	pos makc.Point,
	hasPos bool,
	includeInjected bool,
	normalizeOwnInjected bool,
) error {
	printCapability("relative_move", func() error {
		return client.Mouse.Move(ctx, makc.Rel(0, 0))
	})
	printCapability("absolute_move", func() error {
		if !hasPos {
			return fmt.Errorf("%w: absolute movement requires readable cursor position", makc.ErrUnsupported)
		}
		return client.Mouse.Move(ctx, makc.Abs(pos.X, pos.Y))
	})
	printCapability("listen", func() error {
		listenCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		listener, err := client.Listen(listenCtx, makc.ListenOptions{
			Backend:              listenBackend,
			Mask:                 makc.ListenAll,
			Buffer:               1,
			IncludeInjected:      includeInjected,
			NormalizeOwnInjected: normalizeOwnInjected,
		})
		if err != nil {
			return err
		}
		listener.Close()
		return listener.Wait()
	})
	return nil
}

func printCapability(name string, probe func() error) {
	err := probe()
	if err != nil {
		fmt.Printf("cap_%s=false\n", name)
		fmt.Printf("cap_%s_error=%v\n", name, err)
		return
	}
	fmt.Printf("cap_%s=true\n", name)
}

func runListenSmoke(ctx context.Context, client *makc.Client, backend makc.ListenBackend, count int, includeInjected bool, normalizeOwnInjected bool) error {
	if count <= 0 {
		count = 1
	}

	listenCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	listener, err := client.Listen(listenCtx, makc.ListenOptions{
		Backend:              backend,
		Mask:                 makc.ListenAll,
		Buffer:               32,
		IncludeInjected:      includeInjected,
		NormalizeOwnInjected: normalizeOwnInjected,
	})
	if err != nil {
		return err
	}
	defer listener.Close()

	if includeInjected || normalizeOwnInjected {
		if err := client.Mouse.Move(ctx, makc.Rel(20, 0)); err != nil {
			return err
		}
		if err := client.Keyboard.Tap(ctx, makc.KeyShift); err != nil {
			return err
		}
	}

	seen := 0
	for seen < count {
		select {
		case event, ok := <-listener.Events:
			if !ok {
				return listener.Wait()
			}
			fmt.Printf("event=%s raw=%v device=0x%X injected=%v lower=%v own=%v extra=0x%X %s\n",
				inputEventKindName(event.Kind),
				event.Raw,
				event.Device,
				event.Injected,
				event.LowerIntegrityInjected,
				event.Own,
				event.ExtraInfo,
				inputEventDetail(event),
			)
			seen++
		case <-listenCtx.Done():
			listener.Close()
			_ = listener.Wait()
			fmt.Printf("listen_timeout=true seen=%d\n", seen)
			return nil
		}
	}

	listener.Close()
	if err := listener.Wait(); err != nil {
		return err
	}
	fmt.Printf("listen_seen=%d\n", seen)
	return nil
}

func inputEventKindName(kind makc.InputEventKind) string {
	switch kind {
	case makc.InputEventMouseMove:
		return "mouse_move"
	case makc.InputEventMouseButton:
		return "mouse_button"
	case makc.InputEventMouseWheel:
		return "mouse_wheel"
	case makc.InputEventMouseHWheel:
		return "mouse_hwheel"
	case makc.InputEventKey:
		return "key"
	default:
		return "unknown"
	}
}

func inputEventDetail(event makc.InputEvent) string {
	switch event.Kind {
	case makc.InputEventMouseMove:
		detail := fmt.Sprintf("pos=%d,%d", event.Mouse.Position.X, event.Mouse.Position.Y)
		if event.Raw || event.Mouse.Move.X != 0 || event.Mouse.Move.Y != 0 {
			detail += fmt.Sprintf(" move=%s:%d,%d", moveKind(event.Mouse.Move), event.Mouse.Move.X, event.Mouse.Move.Y)
		}
		return detail
	case makc.InputEventMouseButton:
		return fmt.Sprintf("button=%s state=%s pos=%d,%d", event.Mouse.Button, event.Mouse.State, event.Mouse.Position.X, event.Mouse.Position.Y)
	case makc.InputEventMouseWheel, makc.InputEventMouseHWheel:
		return fmt.Sprintf("delta=%d pos=%d,%d", event.Mouse.Delta, event.Mouse.Position.X, event.Mouse.Position.Y)
	case makc.InputEventKey:
		return fmt.Sprintf("key=%s scan=0x%X state=%s", event.Keyboard.Key, event.Keyboard.ScanCode, event.Keyboard.State)
	default:
		return ""
	}
}

func moveKind(move makc.MouseMove) string {
	if move.Relative {
		return "rel"
	}
	return "abs"
}

func parseKeys(value string) ([]makc.Key, error) {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '+' || r == ',' || r == ' '
	})
	keys := make([]makc.Key, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		key, err := parseKey(part)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("no keys in %q", value)
	}
	return keys, nil
}

func parseKey(name string) (makc.Key, error) {
	return makc.ParseKey(name)
}

func parseBackend(name string) (makc.MouseInjectionBackend, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "auto":
		return makc.MouseInjectionAuto, nil
	case "sendinput":
		return makc.MouseInjectionSendInput, nil
	case "injectmouseinput", "inject":
		return makc.MouseInjectionInjectMouseInput, nil
	case "cgevent", "quartz":
		return makc.MouseInjectionCGEvent, nil
	case "uinput":
		return makc.MouseInjectionUInput, nil
	default:
		return makc.MouseInjectionAuto, fmt.Errorf("unknown backend %q", name)
	}
}

func parseKeyboardBackend(name string) (makc.KeyboardInjectionBackend, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "auto":
		return makc.KeyboardInjectionAuto, nil
	case "sendinput":
		return makc.KeyboardInjectionSendInput, nil
	case "injectkeyboardinput", "inject":
		return makc.KeyboardInjectionInjectKeyboardInput, nil
	case "cgevent", "quartz":
		return makc.KeyboardInjectionCGEvent, nil
	case "uinput":
		return makc.KeyboardInjectionUInput, nil
	default:
		return makc.KeyboardInjectionAuto, fmt.Errorf("unknown keyboard backend %q", name)
	}
}

func parseListenBackend(name string) (makc.ListenBackend, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "auto":
		return makc.ListenBackendAuto, nil
	case "hook", "lowlevelhook", "low-level-hook":
		return makc.ListenBackendLowLevelHook, nil
	case "rawinput", "raw":
		return makc.ListenBackendRawInput, nil
	case "evdev":
		return makc.ListenBackendEvdev, nil
	default:
		return makc.ListenBackendAuto, fmt.Errorf("unknown listen backend %q", name)
	}
}

func parseMovementProfile(name string, steps int, duration time.Duration, jitter int, seed int64) (makc.MovementProfile, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "ease", "easeinout", "ease-in-out":
		return makc.EaseInOutMovement(steps, duration), nil
	case "instant":
		return makc.InstantMovement, nil
	case "linear":
		return makc.LinearMovement(steps, duration), nil
	case "natural":
		return makc.NaturalMovementWithJitter(steps, duration, jitter, seed), nil
	default:
		return makc.MovementProfile{}, fmt.Errorf("unknown movement profile %q", name)
	}
}

func parseInputTag(name string) (uintptr, bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, false, nil
	}
	value, err := strconv.ParseUint(name, 0, 0)
	if err != nil {
		return 0, false, fmt.Errorf("unknown input tag %q", name)
	}
	return uintptr(value), true, nil
}

func parseButton(name string) (makc.MouseButton, error) {
	return makc.ParseMouseButton(name)
}
