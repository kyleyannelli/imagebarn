package web

import (
	"github.com/gofiber/fiber/v2"
	"kmfg.dev/imagebarn/v1/filestore"
)

const IMAGE_ROUTE = "/image"
const BASE_IMAGE = BASE_PARTIAL + "/image"

const ENCODE_MARKER = "#"

var iU *ImageUpload

type ImageUpload struct {
	barnage *BarnageWeb
}

func RegisterUploader(barnage *BarnageWeb) *ImageUpload {
	iU := &ImageUpload{barnage}

	imgRouter := barnage.fiber.Group(IMAGE_ROUTE)
	imgRouter.Use(jwtMiddleware)

	imgRouter.Post("", filestore.UploadImage)
	imgRouter.Get("/:fileName", filestore.GetImage)
	imgRouter.Get("/hash/dir", filestore.HashImageDir)
	imgRouter.Delete("/:fileName", filestore.DeleteImage)

	return iU
}

func jwtMiddleware(c *fiber.Ctx) error {
	email, valid := getEmailFromJWT(c.Cookies("jwt", ""))
	if !valid || !barnage.fs.ApprovedUsers().IsApproved(email) {
		return c.SendStatus(401)
	}
	c.Locals("email", email)
	return c.Next()
}
