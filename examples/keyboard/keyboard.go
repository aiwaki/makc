package main

import (
	"context"
	"log"

	"github.com/NeuralTeam/makc"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := makc.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	log.Printf("keyboard injection backend: %s", client.Keyboard.InjectionBackend())
	log.Printf("input tag: 0x%X", client.InputTag())

	key, err := makc.ParseKey("shift")
	if err != nil {
		log.Fatal(err)
	}
	if err := client.Keyboard.Tap(ctx, key); err != nil {
		log.Fatal(err)
	}
	if err := client.Keyboard.ScanTap(ctx, 0x2A, false); err != nil {
		log.Fatal(err)
	}

	down, err := client.Keyboard.Down(ctx, makc.KeyA)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%s down=%v", makc.KeyA, down)
}
