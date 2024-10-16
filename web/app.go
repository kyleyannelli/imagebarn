package web

import (
	"embed"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/template/html/v2"
	"kmfg.dev/imagebarn/v1/filestore"
)

const MAIN_LAYOUT = "views/layouts/main"
const BASE_VIEW = "views"
const BASE_PARTIAL = BASE_VIEW + "/partials"

const INDEX_ROUTE = "/"
const INDEX_AS_PARTIAL_ROUTE = "/index-as-partial"
const LOGOUT_ROUTE = "/logout"
const PARTIALS_ROUTE = "/partials"
const PARTIALS_IMAGES_ROUTE = PARTIALS_ROUTE + "/images"

const INDEX_VIEW = BASE_VIEW + "/index"
const PARTIALS_IMAGES_VIEW = BASE_PARTIAL + "/images"

const MAX_IMAGES_PER_USER = 5

//go:embed static/*
var staticFS embed.FS

//go:embed views/*
var viewsFS embed.FS

var AdminUserEmail string
var barnage *BarnageWeb

func StartServer(port int, stopChan chan struct{}, wg *sync.WaitGroup) {

	engine := html.NewFileSystem(http.FS(viewsFS), ".html")
	engine.Reload(false)
	engine.Delims("{{", "}}")

	imageUploadSizeLimitMb, err := strconv.Atoi(os.Getenv("UPLOAD_LIMIT_MB"))
	if err != nil {
		panic(fmt.Errorf("Cannot use %v MB for image upload size! Double check your .env is formatted correctly: %v", os.Getenv("UPLOAD_LIMIT_MB"), err))
	}
	app := fiber.New(fiber.Config{
		Views:                   engine,
		ServerHeader:            "ImageBarn v0.0.0",
		BodyLimit:               imageUploadSizeLimitMb * 1024 * 1024,
		EnableTrustedProxyCheck: true,
		TrustedProxies:          getTrustedProxies(),
	})
	app.Use(logFiber)
	app.Use("/static", filesystem.New(filesystem.Config{
		Root:       http.FS(staticFS),
		PathPrefix: "static",
		Browse:     false,
	}))
	app.Get(INDEX_ROUTE, index)
	app.Get(INDEX_AS_PARTIAL_ROUTE, indexAsPartial)
	app.Get(LOGOUT_ROUTE, logout)
	app.Get(PARTIALS_IMAGES_ROUTE, partialsImages)

	filestore.SetupImageConverterWorker()
	StartJWTServices(stopChan, wg)
	barnage = NewBarnage(app, stopChan, wg)
	InitOAuth(barnage)
	RegisterUploader(barnage)
	RegisterApprover(barnage)
	RegisterApi(app)

	// PLEASE REVERSE PROXY AND USE HTTPS
	app.Listen(fmt.Sprintf("127.0.0.1:%v", port))
}

func logout(c *fiber.Ctx) error {
	email, valid := getEmailFromJWT(c.Cookies("jwt", ""))
	if valid {
		InvalidateJwt(email)
	}
	c.Cookie(&fiber.Cookie{
		Name:     "jwt",
		Value:    "",
		Expires:  time.Now(),
		HTTPOnly: true,
		Secure:   isSecure,
	})
	return indexAsPartial(c)
}

func partialsImages(c *fiber.Ctx) error {
	email, valid := getEmailFromJWT(c.Cookies("jwt", ""))
	if !valid {
		return fmt.Errorf("Invalid JWT!")
	}
	return c.Render(PARTIALS_IMAGES_VIEW, fiber.Map{"BarnageUser": getBarnageUser(email)})
}

func indexAsPartial(c *fiber.Ctx) error {
	email, valid := getEmailFromJWT(c.Cookies("jwt", ""))
	if !valid {
		return c.Render(INDEX_VIEW, fiber.Map{})
	}
	return c.Render(INDEX_VIEW, fiber.Map{"BarnageUser": getBarnageUser(email), "ImageUploadRoute": IMAGE_ROUTE})
}

func index(c *fiber.Ctx) error {
	email, valid := getEmailFromJWT(c.Cookies("jwt", ""))
	if !valid {
		return c.Render(INDEX_VIEW, fiber.Map{}, MAIN_LAYOUT)
	}
	return c.Render(INDEX_VIEW, fiber.Map{"BarnageUser": getBarnageUser(email), "ImageUploadRoute": IMAGE_ROUTE}, MAIN_LAYOUT)
}

func logFiber(c *fiber.Ctx) error {
	err := c.Next()
	if err != nil {
		slog.Error(fmt.Sprintf("Error in route %v: %v", c.OriginalURL(), err))
	}
	return err
}

func getTrustedProxies() []string {
	commaSeparatedProxies := strings.ReplaceAll(os.Getenv("TRUSTED_PROXIES"), " ", "")
	proxiesSlice := strings.Split(commaSeparatedProxies, ",")
	proxiesSlice = append(proxiesSlice, "127.0.0.1")
	proxiesSlice = append(proxiesSlice, "::1")
	for i := range proxiesSlice {
		if proxiesSlice[i] == "" {
			continue
		}
		if net.ParseIP(proxiesSlice[i]) == nil {
			panic(fmt.Errorf("IP Address \"%s\" is not valid! Double check your TRUSTED_PROXIES list.", proxiesSlice[i]))
		}
	}
	return proxiesSlice
}
