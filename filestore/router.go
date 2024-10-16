package filestore

import (
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
)

func HashImageDir(c *fiber.Ctx) error {
	email := c.Locals("email").(string)
	hash := imagesDirHash(Encode(email))
	if hash == 0 {
		return c.SendStatus(204)
	}
	return c.SendString(fmt.Sprintf("%d", hash))
}

func GetImage(c *fiber.Ctx) error {
	email := c.Locals("email").(string)
	unescapedFileName, err := url.PathUnescape(c.Params("fileName", ""))
	if err != nil {
		return err
	}
	encFileName := Encode(unescapedFileName)
	filePath := fmt.Sprintf(IMAGES_DIR, Encode(email), encFileName)
	return c.SendFile(url.PathEscape(filePath))
}

func UploadImage(c *fiber.Ctx) error {
	email := c.Locals("email").(string)
	file, err := c.FormFile("image")
	if err != nil {
		return err
	}
	needToConvert := false
	fileType := getHeaderIfAccepted(file.Header)
	if fileType == "" {
		return c.SendStatus(400)
	}
	needToConvert = fileType == "image/heic"

	wg.Add(1)
	defer wg.Done()
	err = os.MkdirAll(fmt.Sprintf(IMAGES_DIR, Encode(email), ""), 0700)
	if err != nil {
		return err
	}
	filePath := fmt.Sprintf(IMAGES_DIR, Encode(email), Encode(file.Filename))
	err = c.SaveFile(file, filePath)
	if err != nil {
		return err
	}

	if needToConvert {
		Semaphore <- struct{}{}

		defer func() { <-Semaphore }()

		// not necessary havent thought about this much
		time.Sleep(1 * time.Second)
		convertHeicToWebp(filePath, file.Filename, Encode(email))
	}
	return c.SendStatus(201)
}

func DeleteImage(c *fiber.Ctx) error {
	email := c.Locals("email").(string)
	unescapedFileName, err := url.PathUnescape(c.Params("fileName", ""))
	if err != nil {
		return err
	}
	err = os.Remove(fmt.Sprintf("./images/%v/%v", Encode(email), Encode(unescapedFileName)))
	if err != nil {
		return err
	}
	return c.SendStatus(200)
}
