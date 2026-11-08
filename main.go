// Package main is the root entrypoint for MyGo desktop tooling (go tool mygo dev / build).
// It delegates directly to internal/client. For the standard CLI layout, see cmd/client and cmd/server.
package main

import (
	"mygo-app/internal/client"
)

func main() {
	client.Run()
}
