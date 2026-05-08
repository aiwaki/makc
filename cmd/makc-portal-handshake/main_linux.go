//go:build linux

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	portalName         = "org.freedesktop.portal.Desktop"
	portalPath         = "/org/freedesktop/portal/desktop"
	remoteDesktopIFace = "org.freedesktop.portal.RemoteDesktop"
	requestIFace       = "org.freedesktop.portal.Request"
	sessionIFace       = "org.freedesktop.portal.Session"

	deviceKeyboard    uint32 = 1
	devicePointer     uint32 = 2
	deviceTouchscreen uint32 = 4
)

func main() {
	var selectDevices bool
	var start bool
	var devices string
	var parentWindow string
	var timeout time.Duration

	flag.BoolVar(&selectDevices, "select-devices", false, "call SelectDevices after CreateSession")
	flag.BoolVar(&start, "start", false, "call Start after SelectDevices; may show a desktop permission prompt")
	flag.StringVar(&devices, "devices", "keyboard,pointer", "device types for SelectDevices: keyboard,pointer,touchscreen or numeric mask")
	flag.StringVar(&parentWindow, "parent-window", "", "parent window identifier passed to Start")
	flag.DurationVar(&timeout, "timeout", 5*time.Second, "portal request timeout")
	flag.Parse()

	if start {
		selectDevices = true
	}

	mask, err := parseDeviceMask(devices)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := run(ctx, selectDevices, start, mask, parentWindow); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, selectDevices bool, start bool, deviceMask uint32, parentWindow string) error {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return fmt.Errorf("connect session bus: %w", err)
	}
	defer conn.Close()

	signals := make(chan *dbus.Signal, 16)
	conn.Signal(signals)
	if err := conn.AddMatchSignal(
		dbus.WithMatchInterface(requestIFace),
		dbus.WithMatchMember("Response"),
		dbus.WithMatchPathNamespace(dbus.ObjectPath(portalPath+"/request")),
	); err != nil {
		return fmt.Errorf("add portal response match: %w", err)
	}
	defer conn.RemoveMatchSignal(
		dbus.WithMatchInterface(requestIFace),
		dbus.WithMatchMember("Response"),
		dbus.WithMatchPathNamespace(dbus.ObjectPath(portalPath+"/request")),
	)

	portal := conn.Object(portalName, dbus.ObjectPath(portalPath))
	requestToken := makeToken("makccreate")
	sessionToken := makeToken("makcsession")

	createRequest, err := callCreateSession(ctx, portal, requestToken, sessionToken)
	if err != nil {
		return err
	}
	fmt.Printf("portal_create_request=%s\n", createRequest)

	createResp, err := waitPortalResponse(ctx, signals, createRequest)
	if err != nil {
		return err
	}
	fmt.Printf("portal_create_response=%d\n", createResp.Code)
	fmt.Printf("portal_create_response_name=%s\n", responseName(createResp.Code))
	printResults("portal_create_result", createResp.Results)
	sessionPath, err := createResp.objectPath("session_handle")
	if err != nil {
		if createResp.Code != 0 {
			return fmt.Errorf("CreateSession response code %d", createResp.Code)
		}
		return err
	}
	fmt.Printf("portal_session_handle=%s\n", sessionPath)
	defer closeSession(conn, sessionPath)
	if createResp.Code != 0 {
		return fmt.Errorf("CreateSession response code %d", createResp.Code)
	}

	if !selectDevices {
		fmt.Println("portal_select_devices_skipped=true")
		fmt.Println("portal_start_skipped=true")
		return nil
	}

	selectRequest, err := callSelectDevices(ctx, portal, sessionPath, makeToken("makcselect"), deviceMask)
	if err != nil {
		return err
	}
	fmt.Printf("portal_select_devices_request=%s\n", selectRequest)
	selectResp, err := waitPortalResponse(ctx, signals, selectRequest)
	if err != nil {
		return err
	}
	fmt.Printf("portal_select_devices_response=%d\n", selectResp.Code)
	fmt.Printf("portal_select_devices_response_name=%s\n", responseName(selectResp.Code))
	printResults("portal_select_devices_result", selectResp.Results)
	if selectResp.Code != 0 {
		return fmt.Errorf("SelectDevices response code %d", selectResp.Code)
	}

	if !start {
		fmt.Println("portal_start_skipped=true")
		return nil
	}

	fmt.Println("portal_start_requested=true")
	startRequest, err := callStart(ctx, portal, sessionPath, makeToken("makcstart"), parentWindow)
	if err != nil {
		return err
	}
	fmt.Printf("portal_start_request=%s\n", startRequest)
	startResp, err := waitPortalResponse(ctx, signals, startRequest)
	if err != nil {
		return err
	}
	fmt.Printf("portal_start_response=%d\n", startResp.Code)
	fmt.Printf("portal_start_response_name=%s\n", responseName(startResp.Code))
	printResults("portal_start_result", startResp.Results)
	if startResp.Code != 0 {
		return fmt.Errorf("Start response code %d", startResp.Code)
	}
	return nil
}

func callCreateSession(ctx context.Context, portal dbus.BusObject, token string, sessionToken string) (dbus.ObjectPath, error) {
	options := map[string]dbus.Variant{
		"handle_token":         dbus.MakeVariant(token),
		"session_handle_token": dbus.MakeVariant(sessionToken),
	}
	var request dbus.ObjectPath
	if err := portal.CallWithContext(ctx, remoteDesktopIFace+".CreateSession", 0, options).Store(&request); err != nil {
		return "", fmt.Errorf("CreateSession: %w", err)
	}
	return request, nil
}

func callSelectDevices(ctx context.Context, portal dbus.BusObject, sessionPath dbus.ObjectPath, token string, deviceMask uint32) (dbus.ObjectPath, error) {
	options := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(token),
		"types":        dbus.MakeVariant(deviceMask),
	}
	var request dbus.ObjectPath
	if err := portal.CallWithContext(ctx, remoteDesktopIFace+".SelectDevices", 0, sessionPath, options).Store(&request); err != nil {
		return "", fmt.Errorf("SelectDevices: %w", err)
	}
	return request, nil
}

func callStart(ctx context.Context, portal dbus.BusObject, sessionPath dbus.ObjectPath, token string, parentWindow string) (dbus.ObjectPath, error) {
	options := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(token),
	}
	var request dbus.ObjectPath
	if err := portal.CallWithContext(ctx, remoteDesktopIFace+".Start", 0, sessionPath, parentWindow, options).Store(&request); err != nil {
		return "", fmt.Errorf("Start: %w", err)
	}
	return request, nil
}

type portalResponse struct {
	Code    uint32
	Results map[string]dbus.Variant
}

func waitPortalResponse(ctx context.Context, signals <-chan *dbus.Signal, request dbus.ObjectPath) (portalResponse, error) {
	for {
		select {
		case <-ctx.Done():
			return portalResponse{}, ctx.Err()
		case sig := <-signals:
			if sig == nil || sig.Path != request || sig.Name != requestIFace+".Response" {
				continue
			}
			if len(sig.Body) != 2 {
				return portalResponse{}, fmt.Errorf("portal response body has %d fields", len(sig.Body))
			}
			code, ok := sig.Body[0].(uint32)
			if !ok {
				return portalResponse{}, fmt.Errorf("portal response code has type %T", sig.Body[0])
			}
			results, ok := sig.Body[1].(map[string]dbus.Variant)
			if !ok {
				return portalResponse{}, fmt.Errorf("portal response results have type %T", sig.Body[1])
			}
			return portalResponse{Code: code, Results: results}, nil
		}
	}
}

func (r portalResponse) objectPath(name string) (dbus.ObjectPath, error) {
	value, ok := r.Results[name]
	if !ok {
		return "", fmt.Errorf("portal response missing %q", name)
	}
	path, ok := value.Value().(dbus.ObjectPath)
	if !ok {
		if raw, stringOK := value.Value().(string); stringOK {
			path = dbus.ObjectPath(raw)
		} else {
			return "", fmt.Errorf("portal response %q has type %T", name, value.Value())
		}
	}
	if !path.IsValid() {
		return "", fmt.Errorf("portal response %q is invalid: %s", name, path)
	}
	return path, nil
}

func responseName(code uint32) string {
	switch code {
	case 0:
		return "success"
	case 1:
		return "cancelled"
	case 2:
		return "other"
	default:
		return "unknown"
	}
}

func printResults(prefix string, results map[string]dbus.Variant) {
	if len(results) == 0 {
		return
	}
	for key, value := range results {
		fmt.Printf("%s_%s=%v\n", prefix, key, value.Value())
	}
}

func closeSession(conn *dbus.Conn, sessionPath dbus.ObjectPath) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = conn.Object(portalName, sessionPath).CallWithContext(ctx, sessionIFace+".Close", 0)
	fmt.Println("portal_session_closed=true")
}

func makeToken(prefix string) string {
	return prefix + strconv.FormatInt(time.Now().UnixNano(), 10) + strconv.Itoa(os.Getpid())
}

func parseDeviceMask(value string) (uint32, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, errors.New("devices cannot be empty")
	}
	if n, err := strconv.ParseUint(value, 0, 32); err == nil {
		return uint32(n), nil
	}

	var mask uint32
	for _, part := range strings.Split(value, ",") {
		switch strings.ToLower(strings.TrimSpace(part)) {
		case "", "none":
		case "keyboard", "key", "keys":
			mask |= deviceKeyboard
		case "pointer", "mouse":
			mask |= devicePointer
		case "touch", "touchscreen":
			mask |= deviceTouchscreen
		default:
			return 0, fmt.Errorf("unknown device type %q", part)
		}
	}
	if mask == 0 {
		return 0, fmt.Errorf("no device types selected in %q", value)
	}
	return mask, nil
}
