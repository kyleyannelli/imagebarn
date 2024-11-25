package filestore

import (
	"fmt"
	"net/url"
	"os"

	"github.com/gofiber/fiber/v2"
	"gopkg.in/h2non/bimg.v1"
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

	fileType := getHeaderIfAccepted(file.Header)
	if fileType == "" {
		return c.SendStatus(400)
	}

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

	Semaphore <- struct{}{}
	defer func() { <-Semaphore }()

	err = compressAndResizeImage(filePath, file.Filename, Encode(email))
	if err != nil {
		return err
	}

	return c.SendStatus(201)
}

func compressAndResizeImage(filePath, fileNameUnecoded, userFolder string) error {
	fileBytes, err := bimg.Read(filePath)
	if err != nil {
		return err
	}

	image := bimg.NewImage(fileBytes)
	size, err := image.Size()
	if err != nil {
		return err
	}

	const maxWidth = 1280
	const maxHeight = 480

	targetWidth := size.Width

	if size.Width > maxWidth || size.Height > maxHeight {
		widthScale := float64(maxWidth) / float64(size.Width)
		heightScale := float64(maxHeight) / float64(size.Height)
		scale := min(widthScale, heightScale)

		targetWidth = int(float64(size.Width) * scale)
	}

	options := bimg.Options{
		Lossless: false,
		Type:     bimg.WEBP,
		Quality:  25,
		Width:    targetWidth,
	}

	newImgBytes, err := image.Process(options)
	if err != nil {
		return err
	}

	newFilenameEnc := Encode(fmt.Sprintf("%v.webp", fileNameUnecoded))
	outputPath := fmt.Sprintf("./images/%v/%v", userFolder, newFilenameEnc)
	err = bimg.Write(outputPath, newImgBytes)
	if err != nil {
		return err
	}

	err = os.Remove(filePath)
	if err != nil {
		return err
	}

	return nil
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
