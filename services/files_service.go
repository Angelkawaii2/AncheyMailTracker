package services

import (
	"errors"
	"image"
	"image/jpeg"
	"log"
	"mailtrackerProject/models"
	"mime/multipart"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

type FilesService struct{ dataDir string }

func NewFilesService(dataDir string) *FilesService { return &FilesService{dataDir: dataDir} }

func (s *FilesService) SaveImage(key string, file multipart.File, removeExif bool) (string, error) {
	if !models.ValidKey(key) {
		return "", errors.New("invalid key format")
	}
	// Ensure per-key directory exists
	dir := filepath.Join(s.dataDir, "entries", key, "images")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Println(err)
		return "", err
	}

	// Decode image (支持 jpg/png/gif/webp等)
	img, format, err := image.Decode(file)
	if err != nil {
		log.Println(err, format)
		return "", err
	}
	log.Printf("image uploaded. format=%s", format)

	// Use UUID for filename, enforce .jpg suffix
	name := uuid.New().String() + ".jpg"
	abs := filepath.Join(dir, name)

	// 打开目标文件
	dst, err := os.Create(abs)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	// 以 JPEG 压缩保存
	opts := &jpeg.Options{Quality: 85}
	if err := jpeg.Encode(dst, img, opts); err != nil {
		return "", err
	}

	// Return URL path under /files so it can be served
	return name, nil
}
