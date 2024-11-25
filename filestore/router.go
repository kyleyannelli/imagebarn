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

	const largerDimension = 1280
	const smallerDimension = 480

	targetWidth, targetHeight := size.Width, size.Height
	if size.Width > largerDimension || size.Height > largerDimension {
		scale := float64(largerDimension) / float64(max(size.Width, size.Height))
		targetWidth = int(float64(size.Width) * scale)
		targetHeight = int(float64(size.Height) * scale)
	} else if size.Width > smallerDimension || size.Height > smallerDimension {
		scale := float64(smallerDimension) / float64(max(size.Width, size.Height))
		targetWidth = int(float64(size.Width) * scale)
		targetHeight = int(float64(size.Height) * scale)
	}

	options := bimg.Options{
		Lossless: false,
		Type:     bimg.WEBP,
		Quality:  25,
		Width:    targetWidth,
		Height:   targetHeight,
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
