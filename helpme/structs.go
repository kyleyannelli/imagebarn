package helpme

import (
	"sync"
)

const MAX_IMAGES_PER_USER = 5

type Alike struct {
	String string
	Score  float32
}

type ApprovedUsers struct {
	users   map[string]bool
	rwMutex sync.RWMutex
}

type AuthUser struct {
	email  string
	Images *[MAX_IMAGES_PER_USER]string
}

func NewAuthUser(email string, images *[MAX_IMAGES_PER_USER]string) *AuthUser {
	return &AuthUser{email, images}
}

func (au *AuthUser) Email() string {
	return au.email
}
