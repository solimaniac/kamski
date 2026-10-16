package main

import (
	"context"
	"fmt"
	"kamski/internal/app"
	"kamski/internal/safety"
	"os"
	"os/signal"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := app.Benchmark(ctx, os.Args[1:], os.Stdout); err != nil {
		keys := []string{os.Getenv("OPENROUTER_API_KEY")}
		for i, a := range os.Args {
			if a == "--api-key" && i+1 < len(os.Args) {
				keys = append(keys, os.Args[i+1])
			}
		}
		fmt.Fprintln(os.Stderr, (safety.Redactor{Secrets: keys}).Text(err.Error()))
		os.Exit(1)
	}
}
