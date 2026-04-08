package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gofiber/fiber/v3"

	"next-gen-ai-web-application/executor/src/handlers/python"
)

func main() {
	app := fiber.New()

	python.RegisterHandlers(app)

	if socketPath := os.Getenv("SOCKET_PATH"); socketPath != "" {
		log.Fatalf("server shutdown. error: %v\n", app.Listen(socketPath, fiber.ListenConfig{
			ListenerNetwork:    fiber.NetworkUnix,
			UnixSocketFileMode: 0777,
		}))
	} else {
		host := os.Getenv("HOST")
		port := os.Getenv("PORT")
		if port == "" {
			port = "6173"
		}

		address := fmt.Sprintf("%s:%s", host, port)

		log.Fatalf("server shutdown. error: %v\n", app.Listen(address, fiber.ListenConfig{
			ListenerNetwork: fiber.NetworkTCP,
		}))
	}
}
