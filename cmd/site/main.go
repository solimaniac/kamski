package main

import (
	"fmt"
	"os"

	"kamski/internal/app/sitecli"
	"kamski/internal/safety"
)

func main() {
	if err := sitecli.Run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, (safety.Redactor{Secrets: []string{os.Getenv("OPENROUTER_API_KEY")}}).Text(err.Error()))
		os.Exit(1)
	}
}
