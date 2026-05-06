package main

import (
	"context"
	"log"
	"time"

	"github.com/aiwaki/makc"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := makc.Open(makc.WithMouseInjection(makc.MouseInjectionAuto))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	log.Printf("mouse injection backend: %s", client.Mouse.InjectionBackend())
	log.Printf("input tag: 0x%X", client.InputTag())

	start, err := client.Mouse.Position(ctx)
	if err != nil {
		log.Fatal(err)
	}
	screen, err := client.Mouse.ScreenSize(ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("screen=%dx%d position=%d,%d", screen.X, screen.Y, start.X, start.Y)

	listener, err := client.Listen(ctx, makc.ListenOptions{
		Mask:            makc.ListenMouse,
		Buffer:          8,
		IncludeInjected: true,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	if err := client.Mouse.Inject(ctx,
		makc.MouseMoveEvent(makc.Rel(10, 10)),
		makc.MousePauseEvent(50*time.Millisecond),
		makc.MouseMoveEvent(makc.Abs(start.X, start.Y)),
	); err != nil {
		log.Fatal(err)
	}

	select {
	case event := <-listener.Events:
		log.Printf("mouse event kind=%d injected=%v own=%v extra=0x%X pos=%d,%d",
			event.Kind,
			event.Injected,
			event.Own,
			event.ExtraInfo,
			event.Mouse.Position.X,
			event.Mouse.Position.Y,
		)
	case <-ctx.Done():
		log.Fatal(ctx.Err())
	}
}
