package web

import (
	"sync"

	"github.com/gofiber/fiber/v2"
	"kmfg.dev/imagebarn/v1/filestore"
	"kmfg.dev/imagebarn/v1/helpme"
)

type BarnageUser struct {
	authUser   *helpme.AuthUser
	Email      string
	IsApproved bool
	IsAdmin    bool
}

type BarnageWeb struct {
	fiber *fiber.App
	fs    *filestore.Filestore
}

func NewBarnage(fiber *fiber.App, stopChan chan struct{}, wg *sync.WaitGroup) *BarnageWeb {
	fs := filestore.NewFilestore(AdminUserEmail, wg)
	fs.StoreApprovedUsersRoutine(stopChan)
	return &BarnageWeb{fiber, fs}
}

func IsApproved(authUser *helpme.AuthUser) bool {
	return barnage.fs.IsApproved(authUser)
}

func IsAdmin(authUser *helpme.AuthUser) bool {
	return authUser.Email() == AdminUserEmail
}

func (barnUser *BarnageUser) Images() *[helpme.MAX_IMAGES_PER_USER]string {
	return barnUser.authUser.Images
}

func (barnUser *BarnageUser) MaxedOut() bool {
	return barnUser.ActualImageCount() >= MAX_IMAGES_PER_USER
}

func (barnUser *BarnageUser) ActualImageCount() int {
	totalNonEmpty := 0
	for i := range barnUser.authUser.Images {
		if barnUser.authUser.Images[i] != "" {
			totalNonEmpty++
		}
	}
	return totalNonEmpty
}

func (barnUser *BarnageUser) NoImages() bool {
	return barnUser.authUser.Images == nil || barnUser.ActualImageCount() == 0
}

func getBarnageUser(email string) *BarnageUser {
	authUser := barnage.fs.GetAuthUser(email)
	return &BarnageUser{
		authUser,
		authUser.Email(),
		IsApproved(authUser),
		IsAdmin(authUser),
	}
}
