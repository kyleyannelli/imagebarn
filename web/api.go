package web

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

var authToken string

func RegisterApi(fiber *fiber.App) {
	authToken = os.Getenv("BEARER_TOKEN")
	if authToken == "" {
		panic("Bearer Token was not provided in the .env! This makes ImageBarn useless. Please double check the presence of BEARER_TOKEN= in your .env")
	}
	apiRouter := fiber.Group("/api")
	apiRouter.Use(limiter.New(limiter.Config{
		Max:               60,
		Expiration:        1 * time.Minute,
		LimiterMiddleware: limiter.SlidingWindow{},
	}))
	apiRouter.Use(authHeaderMiddleware)
	apiRouter.Get("/image", getImageThenRemove)
}

func authHeaderMiddleware(c *fiber.Ctx) error {
	c.Response().Header.Add("Access-Control-Allow-Origin", "*")
	c.Response().Header.Add("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	c.Response().Header.Add("Access-Control-Allow-Headers", "Content-Type, Authorization")
	c.Response().Header.Add("Access-Control-Allow-Credentials", "true")

	if c.Method() == "OPTIONS" {
		return c.SendStatus(204)
	}

	authBytes := c.Request().Header.Peek("Authorization")
	doesntMatchToken := string(authBytes[:]) != "Bearer "+authToken
	if doesntMatchToken {
		return c.SendStatus(401)
	}
	return c.Next()
}

func getImageThenRemove(c *fiber.Ctx) error {
	pickedDirectory, pickedFile, err := barnage.fs.GetRandomImage()
	if err != nil {
		slog.Debug(fmt.Sprintf("Couldn't find any images to ghost: %v", err))
		// no content available
		return c.SendStatus(204)
	}
	fullPath := fmt.Sprintf("./images/%v/%v", pickedDirectory, pickedFile)
	escapedPickedFile := url.PathEscape(fullPath)
	sendFileErr := c.SendFile(escapedPickedFile)
	err = barnage.fs.GhostImage(pickedDirectory, pickedFile)
	if err != nil {
		return err
	}
	return sendFileErr
}
