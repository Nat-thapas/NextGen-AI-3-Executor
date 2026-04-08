package python

import (
	"github.com/gofiber/fiber/v3"
)

func RegisterHandlers(app *fiber.App) {
	python := app.Group("/python")
	python.Post("/execute", Execute)
}
