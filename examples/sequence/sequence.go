package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/aiwaki/makc"
)

func main() {
	var click bool
	var text string
	var seed int64

	flag.BoolVar(&click, "click", false, "include a real left-click step")
	flag.StringVar(&text, "text", "", "optional text to type")
	flag.Int64Var(&seed, "seed", 42, "input profile seed")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := makc.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	profile := makc.BalancedInputProfile(seed)
	sequence := makc.NewInputSequence(
		makc.MoveStep(makc.Rel(12, 0)),
		makc.PauseStep(80*time.Millisecond),
		makc.MoveStep(makc.Rel(-12, 0)),
	)

	if click {
		sequence = sequence.Append(makc.ClickStep(makc.ButtonLeft, profile.Click))
	}
	if text != "" {
		sequence = sequence.Append(makc.TextStep(text, profile.Typing))
	}

	log.Printf("mouse injection backend: %s", client.Mouse.InjectionBackend())
	log.Printf("keyboard injection backend: %s", client.Keyboard.InjectionBackend())
	log.Printf("sequence steps: %d", len(sequence.Steps))

	if err := client.Run(ctx, sequence); err != nil {
		log.Fatal(err)
	}
}
