package main

import (
	"context"
	"fmt"
	"kamski/internal/app"
	"kamski/internal/safety"
	"os"
	"os/signal"
	"strings"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := app.Benchmark(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, errorRedactor(os.Args[1:]).Text(err.Error()))
		os.Exit(1)
	}
}

// Scan even after a flag parse failure, which may occur before app can resolve
// the credential. flag accepts both single- and double-dash spellings.
func errorRedactor(args []string) safety.Redactor {
	keys := []string{os.Getenv("OPENROUTER_API_KEY")}
	for i, arg := range args {
		arg = strings.TrimLeft(arg, "-")
		if strings.HasPrefix(arg, "api-key=") {
			keys = append(keys, strings.TrimPrefix(arg, "api-key="))
		}
		if arg == "api-key" && i+1 < len(args) {
			keys = append(keys, args[i+1])
		}
	}
	return safety.Redactor{Secrets: keys}
}
