package filestore

import (
	"crypto/rand"
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"log/slog"
	"math/big"
	psuedoRand "math/rand"
	"net/textproto"
	"os"
	"strconv"
	"strings"

	"gopkg.in/h2non/bimg.v1"
	"kmfg.dev/imagebarn/v1/helpme"
)

const IMAGES_DIR = "./images/%v/%v"

func (fs *Filestore) GetAuthUser(email string) *helpme.AuthUser {
	authUser := helpme.NewAuthUser(email, nil)
	err := fs.GatherImages(authUser)
	if err != nil {
		slog.Warn(fmt.Sprintf("Couldn't gather images for %v: %v", email, err))
	}
	return authUser
}

// not really sure if this is necessary,
//
//	but it's technically a user provided string accessing the filesystem.
func Encode(raw string) string {
	return fmt.Sprintf("%v#%v", len([]rune(raw)), raw)
}

func Decode(encoded string) (string, error) {
	encodeMarkerIdx, strLen, err := canDecode(encoded)
	if err != nil {
		return "", err
	}

	afterEncodeMarkerIdx := encodeMarkerIdx + 1
	runes := []rune(encoded[afterEncodeMarkerIdx:])
	if len(runes) < strLen {
		return "", fmt.Errorf("Length mismatch: expected %d characters, but got %d", strLen, len(runes))
	}

	decodedRunes := runes[:strLen]
	return string(decodedRunes), nil
}

// returns encodeMarkerIdx, strLen, err
func canDecode(encoded string) (int, int, error) {
	encodeMarkerIdx := strings.Index(encoded, "#")
	if encodeMarkerIdx == -1 {
		return -1, -1, fmt.Errorf("Failed to find encoder marker '#'")
	}

	strLen, err := strconv.Atoi(encoded[:encodeMarkerIdx])
	if err != nil {
		return -1, -1, fmt.Errorf("Decoding failed. Could not convert \"%v\" to integer: %v", encoded[:encodeMarkerIdx], err)
	}

	return encodeMarkerIdx, strLen, nil
}

func ReadDir(authUser *helpme.AuthUser) ([]fs.DirEntry, error) {
	return os.ReadDir(fmt.Sprintf(IMAGES_DIR, Encode(authUser.Email()), ""))
}

func DeleteAll(email string) error {
	return os.RemoveAll(fmt.Sprintf(IMAGES_DIR, Encode(email), ""))
}

func (fs *Filestore) GatherImages(authUser *helpme.AuthUser) error {
	if !fs.IsApproved(authUser) {
		return fmt.Errorf("User %v is not approved!", authUser.Email())
	}

	dir, err := ReadDir(authUser)
	if err != nil {
		return err
	}

	imageNamesArr := [helpme.MAX_IMAGES_PER_USER]string{}
	imageCount := 0
	for i := 0; i < len(dir); i++ {
		if _, _, err := canDecode(dir[i].Name()); err == nil {
			if imageCount >= helpme.MAX_IMAGES_PER_USER {
				return fmt.Errorf("Images found exceed max allowed images (%v)", helpme.MAX_IMAGES_PER_USER)
			}
			imageNamesArr[imageCount], err = Decode(dir[i].Name())
			if err != nil {
				// panic bc we just said we can decode but failed to decode!
				panic(err)
			}
			imageCount++
		}
	}

	authUser.Images = &imageNamesArr

	return nil
}

func getHeaderIfAccepted(header textproto.MIMEHeader) string {
	contentType, exists := header["Content-Type"]
	if !exists || len(contentType) < 1 {
		return ""
	}
	if contentType[0] == "image/heic" ||
		contentType[0] == "image/png" ||
		contentType[0] == "image/jpeg" ||
		contentType[0] == "image/jpg" ||
		contentType[0] == "image/gif" ||
		contentType[0] == "image/webp" {
		return contentType[0]
	}
	return ""
}

func convertHeicToWebp(filePath, fileNameUnecoded, userFolder string) error {
	fileBytes, err := bimg.Read(filePath)
	if err != nil {
		return err
	}

	const maxWidth = 720
	const maxHeight = 480

	image := bimg.NewImage(fileBytes)
	size, err := image.Size()
	if err != nil {
		return err
	}

	width := size.Width
	height := size.Height
	scale := 1.0

	if width > maxWidth || height > maxHeight {
		widthScale := float64(maxWidth) / float64(width)
		heightScale := float64(maxHeight) / float64(height)
		scale = min(widthScale, heightScale)
	}

	resizedWidth := int(float64(width) * scale)
	resizedHeight := int(float64(height) * scale)

	options := bimg.Options{
		Type:     bimg.WEBP,
		Quality:  25,
		Lossless: false,
		Width:    resizedWidth,
		Height:   resizedHeight,
	}

	newImgBytes, err := image.Process(options)
	if err != nil {
		return err
	}

	newFilenameEnc := Encode(fmt.Sprintf("%v.webp", fileNameUnecoded))

	err = bimg.Write(fmt.Sprintf("./images/%v/%v", userFolder, newFilenameEnc), newImgBytes)
	if err != nil {
		return err
	}

	err = os.Remove(filePath)
	if err != nil {
		return err
	}

	return nil
}

func (fs *Filestore) GetRandomImage() (string, string, error) {
	dirContents, err := os.ReadDir("./images")
	if err != nil {
		return "", "", err
	}

	dirNames := []string{}
	for _, dirEntry := range dirContents {
		if dirEntry.IsDir() {
			dirNames = append(dirNames, dirEntry.Name())
		}
	}

	psuedoRand.Shuffle(len(dirNames), func(i, j int) {
		dirNames[i], dirNames[j] = dirNames[j], dirNames[i]
	})

	for _, dirName := range dirNames {
		if _, _, err := canDecode(dirName); err != nil {
			continue
		}

		pickedDirContent, err := os.ReadDir("./images/" + dirName)
		if err != nil {
			slog.Warn(fmt.Sprintf("Couldn't open directory %v: %v", dirName, err))
			continue
		}

		fileNames := []string{}
		for _, fileEntry := range pickedDirContent {
			if !fileEntry.IsDir() {
				fileNames = append(fileNames, fileEntry.Name())
			}
		}

		psuedoRand.Shuffle(len(fileNames), func(i, j int) {
			fileNames[i], fileNames[j] = fileNames[j], fileNames[i]
		})

		for _, fileName := range fileNames {
			if _, _, err := canDecode(fileName); err != nil || isGhostFile(fileName) {
				continue
			}
			return dirName, fileName, nil
		}
	}

	return "", "", fmt.Errorf("No available images found")
}

func (fs *Filestore) GhostImage(directory string, file string) error {
	fullPath := fmt.Sprintf("./images/%v/%v", directory, file)
	originalFile, err := os.Open(fullPath)
	defer originalFile.Close()
	if err != nil {
		return err
	}
	decFilename, err := Decode(file)
	if err != nil {
		return err
	}
	ghostFileFullPath := fmt.Sprintf("./images/%v/%v", directory, Encode(decFilename+".ghost"))
	ghostFile, err := os.Create(ghostFileFullPath)
	defer ghostFile.Close()
	if err != nil {
		return err
	}
	_, err = io.Copy(ghostFile, originalFile)
	if err != nil {
		return err
	}
	err = ghostFile.Sync()
	if err != nil {
		return err
	}

	err = os.Remove(fullPath)
	if err != nil {
		return err
	}
	return err
}

func isDirGhosted(dirName string) bool {
	dir, err := os.ReadDir("./images/" + dirName)
	if err != nil {
		slog.Warn(fmt.Sprintf("Failed to check if dir \"%v\" was ghosted: %v", dirName, err))
		return true
	}
	totalFileCount := 0
	availableFilesCount := 0
	for i := range dir {
		_, _, err := canDecode(dir[i].Name())
		isBarnageFile := err == nil
		if isBarnageFile {
			totalFileCount++
			if !isGhostFile(dir[i].Name()) {
				availableFilesCount++
			}
		}
	}
	return availableFilesCount == 0
}

// return of 0 is considered empty dir or error
func imagesDirHash(dirName string) uint32 {
	dir, err := os.ReadDir("./images/" + dirName)
	if err != nil {
		slog.Debug(err.Error())
		return 0
	}
	totalFiles := ""
	for i := range dir {
		if _, _, err := canDecode(dir[i].Name()); err == nil {
			totalFiles += dir[i].Name()
		}
	}
	if totalFiles == "" {
		return 0
	}
	h := fnv.New32a()
	h.Write([]byte(totalFiles))
	return h.Sum32()
}

func isGhostFile(filename string) bool {
	const ext = ".ghost"
	if len(filename) < len(ext) {
		return false
	}
	return filename[len(filename)-len(ext):] == ext
}

func randInt(max int64) (int64, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(max))
	if err != nil {
		return 0, err
	}
	return n.Int64(), nil
}
