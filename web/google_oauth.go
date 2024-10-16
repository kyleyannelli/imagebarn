package web

import (
	"bytes"
	"crypto/hmac"
	secureRand "crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/goccy/go-json"

	"github.com/gofiber/fiber/v2"
)

const SIGN_IN_VIEW = BASE_VIEW + "/sign-in"
const CALLBACK_ROUTE = "/auth/callback"
const INIT_ROUTE = "/auth/google"

const MAX_SIGN_IN_TIME = 2 * time.Minute
const OAUTH_V2 = "https://accounts.google.com/o/oauth2/v2/auth?"
const GOOGLE_OAUTH_V2_SCOPE = "https://www.googleapis.com/auth/userinfo.email"
const GOOGLE_OAUTH_V2_TOKEN = "https://oauth2.googleapis.com/token"
const GOOGLE_OAUTH_V2_TOKEN_INFO = "https://www.googleapis.com/oauth2/v3/tokeninfo"
const GOOGLE_OAUTH_V2_RESPONSE_TYPE = "code"
const LETTER_BYTES = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

var (
	currentStatesRWMutex = sync.RWMutex{}
	currentStates        = map[string]time.Time{}

	oldSecretRWMutex     = sync.RWMutex{}
	oldSecret            string
	currentSecretRWMutex = sync.RWMutex{}
	currentSecret        string

	areServicesRunning = false

	googleClientId     string
	googleClientSecret string
	baseUri            string
	isSecure           bool = true
)

func init() {
	runCleaner()
}

type GoogleOAuth struct {
	barnage *BarnageWeb
}

func InitOAuth(barnage *BarnageWeb) *GoogleOAuth {
	googleClientId = os.Getenv("GOOGLE_CLIENT_ID")
	googleClientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
	baseUri = os.Getenv("BASE_URI")
	if googleClientId == "" || googleClientSecret == "" || baseUri == "" {
		panic(fmt.Errorf("Incomplete .env file! Please compare your .env with .env.example"))
	}

	if baseUri[0:5] == "http:" {
		isSecure = false
		slog.Warn(fmt.Sprintf("Provided base uri \"%s\" caused cookies to be allowed via HTTP. Only do this in a development environment!", baseUri))
	}

	barnage.fiber.Get(CALLBACK_ROUTE, SignIn)
	barnage.fiber.Get(INIT_ROUTE, StartSignIn)

	return &GoogleOAuth{barnage}
}

func runCleaner() {
	if !areServicesRunning {
		areServicesRunning = true
		go func() {
			for {
				time.Sleep(2 * time.Second)
				cleanStates()
			}
		}()
		rotateSecret()
		go func() {
			for {
				time.Sleep(1 * time.Hour)
				rotateSecret()
			}
		}()
	}
}

func rotateSecret() {
	newSecret := generateSecret()

	currentSecretRWMutex.Lock()
	oldSecretRWMutex.Lock()

	oldSecret = currentSecret
	currentSecret = newSecret

	currentSecretRWMutex.Unlock()
	oldSecretRWMutex.Unlock()
}

func generateSecret() string {
	const SECRET_SIZE = 32
	b := make([]byte, SECRET_SIZE)
	for i := range b {
		randomPos, err := secureRand.Int(secureRand.Reader, big.NewInt(int64(len(LETTER_BYTES))))
		if err != nil {
			panic(err)
		}
		b[i] = LETTER_BYTES[randomPos.Int64()]
	}

	return base64.StdEncoding.EncodeToString(b)
}

func cleanStates() {
	currentStatesRWMutex.Lock()
	defer currentStatesRWMutex.Unlock()
	for state, createdAt := range currentStates {
		if time.Since(createdAt) > MAX_SIGN_IN_TIME {
			delete(currentStates, state)
		}
	}
}

func signState(state string, secret string) string {
	currentSecretRWMutex.RLock()
	h := hmac.New(sha512.New, []byte(secret))
	currentSecretRWMutex.RUnlock()
	h.Write([]byte(state))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// Creates random string, pushes to map, then return the state signed with HMAC
func insertState() (string, string) {
	const STATE_SIZE = 15
	b := make([]byte, STATE_SIZE)
	for i := range b {
		b[i] = LETTER_BYTES[rand.Int63()%int64(len(LETTER_BYTES))]
	}
	state := string(b)

	currentStatesRWMutex.Lock()
	currentStates[state] = time.Now()
	currentStatesRWMutex.Unlock()

	currentSecretRWMutex.RLock()
	signedState := signState(state, currentSecret)
	currentSecretRWMutex.RUnlock()

	return state, signedState
}

func verifyState(unsignedState string, signedState string) bool {
	// verify the state exists & is not expired
	currentStatesRWMutex.Lock()
	createdAt, exists := currentStates[unsignedState]
	if exists {
		delete(currentStates, unsignedState)
		if time.Since(createdAt) > MAX_SIGN_IN_TIME {
			currentStatesRWMutex.Unlock()
			slog.Debug(fmt.Sprintf("Provided signature %v has expired!", unsignedState))
			return false
		}
	} else {
		currentStatesRWMutex.Unlock()
		slog.Debug(fmt.Sprintf("Provided signature %v does not exist!", unsignedState))
		return false
	}
	currentStatesRWMutex.Unlock()

	// verify the given signed state can match the unsigned state with either current or old state
	currentSecretRWMutex.RLock()
	oldSecretRWMutex.RLock()

	signedViaCurrent := signState(unsignedState, currentSecret)
	signedViaOld := signState(unsignedState, oldSecret)
	signedState = strings.ReplaceAll(signedState, " ", "+")
	if signedViaCurrent != signedState &&
		signedViaOld != signedState {
		slog.Debug(fmt.Sprintf("Provided signature %v does not sign!\n\tSigned Current: %v\n\tSigned Old: %v", unsignedState, signedViaCurrent, signedViaOld))
		currentSecretRWMutex.RUnlock()
		oldSecretRWMutex.RUnlock()
		return false
	}
	currentSecretRWMutex.RUnlock()
	oldSecretRWMutex.RUnlock()

	return true
}

func GenerateOAuthLink(state string) string {
	return OAUTH_V2 +
		"client_id=" + googleClientId +
		"&response_type=" + GOOGLE_OAUTH_V2_RESPONSE_TYPE +
		"&scope=" + GOOGLE_OAUTH_V2_SCOPE +
		"&state=" + state +
		"&redirect_uri=" + baseUri + CALLBACK_ROUTE
}

func getAccessTokenFromGoogle(code string) (string, error) {
	urlEncodedData := url.Values{}
	urlEncodedData.Set("code", code)
	urlEncodedData.Set("client_id", googleClientId)
	urlEncodedData.Set("client_secret", googleClientSecret)
	urlEncodedData.Set("grant_type", "authorization_code")
	urlEncodedData.Set("redirect_uri", baseUri+CALLBACK_ROUTE)

	req, err := http.NewRequest("POST", GOOGLE_OAUTH_V2_TOKEN, bytes.NewBufferString(urlEncodedData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err != nil {
		return "", err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var resJson map[string]json.RawMessage
	json.Unmarshal(body, &resJson)

	accessTokenBytes, exists := resJson["access_token"]
	if !exists {
		return "", fmt.Errorf("Access token does not exists!")
	}

	return string(accessTokenBytes), nil
}

func getEmailFromGoogle(accessToken string) (string, error) {
	req, err := http.NewRequest("GET", GOOGLE_OAUTH_V2_TOKEN_INFO, nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", fmt.Sprintf("%s %s", "Bearer ", accessToken))
	if err != nil {
		return "", err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var resJson map[string]json.RawMessage
	json.Unmarshal(body, &resJson)

	accessTokenBytes, exists := resJson["email"]
	if !exists {
		return "", fmt.Errorf("Access token does not exists!")
	}

	return strings.ReplaceAll(string(accessTokenBytes), "\"", ""), nil
}

func SignIn(c *fiber.Ctx) error {
	signedState := c.Query("state", "")
	unsignedState := c.Cookies("state", "")
	haveInvalidStates := signedState == "" || unsignedState == ""
	slog.Debug(fmt.Sprintf("\nStates Provided:\n\tUnsigned: %v\n\tSigned: %v", unsignedState, signedState))

	if haveInvalidStates || !verifyState(unsignedState, signedState) {
		return c.Render(SIGN_IN_VIEW, fiber.Map{"Error": "Took too long to sign in! Please try again."}, MAIN_LAYOUT)
	}

	accessToken, err := getAccessTokenFromGoogle(c.Query("code", ""))
	if err != nil {
		slog.Debug(fmt.Sprintf("Failed to get access token: %v", err))
		return c.Render(SIGN_IN_VIEW, fiber.Map{"Error": "Internal Server Error: Failed to sign in!"}, MAIN_LAYOUT)
	}

	email, err := getEmailFromGoogle(accessToken)
	if err != nil {
		slog.Debug(fmt.Sprintf("Failed to get email: %v", err))
		return c.Render(SIGN_IN_VIEW, fiber.Map{"Error": "Internal Server Error: Failed to sign in!"}, MAIN_LAYOUT)
	}

	jwt, err := CreateJwt(email)
	if err != nil {
		slog.Debug(fmt.Sprintf("Failed to generate JWT: %v", err))
		return c.Render(SIGN_IN_VIEW, fiber.Map{"Error": "Internal Server Error: Failed to sign in!"}, MAIN_LAYOUT)
	}

	c.Cookie(&fiber.Cookie{
		Name:     "jwt",
		Value:    jwt,
		HTTPOnly: true,
		Secure:   isSecure,
	})

	c.Cookie(&fiber.Cookie{
		Name:    "state",
		Value:   "",
		Expires: time.Now(),
		Secure:  isSecure,
	})

	return c.Render(SIGN_IN_VIEW, fiber.Map{}, MAIN_LAYOUT)
}

func StartSignIn(c *fiber.Ctx) error {
	state, signedState := insertState()
	c.Cookie(&fiber.Cookie{
		Name:     "state",
		Value:    state,
		HTTPOnly: true,
		Secure:   isSecure,
	})
	return c.Redirect(GenerateOAuthLink(signedState), 302)
}
