package web

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/goccy/go-json"
	gojwt "github.com/golang-jwt/jwt/v5"
)

const JWT_GOOD_FOR = 90 * 24 * time.Hour
const KEY_FILE = "./ec_private_key.pem"
const ISSUED_VERSION_FILE = "./issued-versions.json"

var issuedVersion = map[string]int{}
var issuedVersionRWMutex = sync.RWMutex{}

var loaded = false

var EC, EC_ERR = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

func StartJWTServices(stopChan chan struct{}, wg *sync.WaitGroup) {
	if !loaded {
		loaded = true
		AdminUserEmail = os.Getenv("ADMIN_USER")
		if AdminUserEmail == "" {
			panic("Missing email in .env for ADMIN_USER=")
		}
		genOrLoadEc()
		loadIssuedVersions()
		storeIssuedVersionsRoutine(stopChan, wg)
	} else {
		slog.Debug("Attempted to start JWT services after they have been started!")
	}
}

func loadIssuedVersions() {
	data, err := os.ReadFile(ISSUED_VERSION_FILE)
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to read file %v: %v", ISSUED_VERSION_FILE, err))
		return
	}

	var issuedVersionTmp map[string]int
	if err = json.Unmarshal(data, &issuedVersionTmp); err != nil {
		panic(fmt.Errorf("Error loading issued versions json: %v", err))
	}

	issuedVersionRWMutex.Lock()
	issuedVersion = issuedVersionTmp
	issuedVersionRWMutex.Unlock()
}

func storeIssuedVersionsRoutine(stopChan chan struct{}, wg *sync.WaitGroup) {
	go func() {
		for {
			wg.Add(1)
			defer wg.Done()
			select {
			case <-stopChan:
				slog.Info("Safely stopping issue storage routine.")
				return
			default:
				time.Sleep(2 * time.Second)
				// why copy data then save?
				//  on devices that use sd card storage this could be super slow to lock for an entire marshal
				issuedVersionRWMutex.RLock()
				copiedIssuedVersion := make(map[string]int, len(issuedVersion))
				for email, version := range issuedVersion {
					copiedIssuedVersion[email] = version
				}
				issuedVersionRWMutex.RUnlock()

				data, err := json.Marshal(copiedIssuedVersion)
				if err != nil {
					slog.Warn(fmt.Sprintf("Failed to marshal issued versions for storge!: %v", err))
					continue
				}

				file, err := os.Create(ISSUED_VERSION_FILE)
				if err != nil {
					slog.Warn(fmt.Sprintf("Failed to create file issued version storge!: %v", err))
					continue
				}
				defer file.Close()

				_, err = file.Write(data)
				if err != nil {
					slog.Warn(fmt.Sprintf("Failed write to file issued version storge!: %v", err))
					continue
				}
			}
		}
	}()
}

func genOrLoadEc() {
	if _, err := os.Stat(KEY_FILE); err == nil {
		loadEcFromFile()
	} else if os.IsNotExist(err) {
		generateEcToFile()
	} else {
		panic(fmt.Sprintf("failed to check key file: %v", err))
	}
}

func generateEcToFile() {
	slog.Info("Key file not found. Using & saving generated...")
	if EC_ERR != nil {
		panic(EC_ERR)
	}

	encodedEC, err := encodePrivateKeyToPEM(EC)
	if err != nil {
		panic(err)
	}

	err = os.WriteFile(KEY_FILE, []byte(encodedEC), 0600)
	if err != nil {
		panic(fmt.Sprintf("failed to write key to file: %v", err))
	}

	err = os.Chmod(KEY_FILE, 0600)
	if err != nil {
		panic(fmt.Sprintf("failed to set file permissions: %v", err))
	}

	slog.Info("Successfully generated and saved EC private key.")
}

func loadEcFromFile() {
	slog.Info("Loading EC private key from file...")

	pemData, err := os.ReadFile(KEY_FILE)
	if err != nil {
		panic(fmt.Sprintf("failed to read key file: %v", err))
	}

	elc, err := decodePrivateKeyFromPEM(pemData)
	if err != nil {
		panic(fmt.Sprintf("failed to decode private key: %v", err))
	}
	EC = elc

	slog.Info("Successfully loaded EC private key.")
}

func decodePrivateKeyFromPEM(pemData []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil || block.Type != "EC PRIVATE KEY" {
		return nil, fmt.Errorf("failed to decode PEM block containing EC private key")
	}
	priv, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse EC private key: %w", err)
	}
	return priv, nil
}

func encodePrivateKeyToPEM(priv *ecdsa.PrivateKey) (string, error) {
	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return "", fmt.Errorf("failed to marshal EC private key: %w", err)
	}

	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: privBytes,
	})

	return string(privPEM), nil
}

func IsValid(jwtStr string) (gojwt.MapClaims, bool) {
	token, err := gojwt.Parse(jwtStr, func(token *gojwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*gojwt.SigningMethodECDSA); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		if EC_ERR != nil {
			return nil, fmt.Errorf("Error with ECSDA gen: %v", EC_ERR)
		}

		return &EC.PublicKey, nil
	})

	if err != nil {
		slog.Debug(fmt.Sprintf("Error parsing token!:\n\t%v", err))
		return nil, false
	}

	claims, ok := token.Claims.(gojwt.MapClaims)

	if token.Valid && ok && claims["expires"] != "" {
		claimExpStr := fmt.Sprint(claims["expires"])
		expires, err := time.Parse(time.RFC3339Nano, claimExpStr)

		if err != nil {
			slog.Error(fmt.Sprintf("Error parsing expires time!:\n\t%v", err))
			return nil, false
		}

		if claims["version"] == nil {
			slog.Error("No version provided in JWT.")
			return nil, false
		}
		claimVersion := int(claims["version"].(float64))

		issuedVersionRWMutex.RLock()
		defer issuedVersionRWMutex.RUnlock()
		if issuedVersion[claims["email"].(string)] == claimVersion {
			return claims, time.Now().Before(expires)
		} else {
			return nil, false
		}
	} else {
		slog.Error(fmt.Sprintf("Error parsing claims!:\n\t%v", err))
		return nil, false
	}
}

func CreateJwt(email string) (string, error) {
	issuedVersionRWMutex.Lock()
	issuedVersion[email]++
	issuedVersionRWMutex.Unlock()

	if EC_ERR != nil {
		return "", EC_ERR
	}

	expireTime := expiresAt()

	issuedVersionRWMutex.RLock()
	token := gojwt.NewWithClaims(gojwt.SigningMethodES256, gojwt.MapClaims{
		"version": issuedVersion[email],
		"expires": expireTime.UTC(),
		"email":   email,
	})
	tokStr, err := token.SignedString(EC)
	issuedVersionRWMutex.RUnlock()

	if err != nil {
		return "", err
	}

	return tokStr, nil
}

func InvalidateJwt(email string) {
	issuedVersionRWMutex.Lock()
	issuedVersion[email]++
	issuedVersionRWMutex.Unlock()
}

func getEmailFromJWT(jwt string) (string, bool) {
	claims, valid := IsValid(jwt)
	if !valid {
		slog.Debug(fmt.Sprintf("Failed to validate JWT"))
		return "", false
	}
	email, exists := claims["email"]
	if !exists {
		slog.Debug(fmt.Sprintf("Email doesn't exist in JWT"))
		return "", false
	}
	return email.(string), true
}

func expiresAt() time.Time {
	return time.Now().Add(JWT_GOOD_FOR)
}
