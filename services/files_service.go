package services

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"mailtrackerProject/models"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"github.com/google/uuid"
	"github.com/kolesa-team/go-webp/decoder"
	"github.com/kolesa-team/go-webp/encoder"
	"github.com/kolesa-team/go-webp/webp"
	"github.com/strukturag/libheif/go/heif"
)

type FilesService struct{ dataDir string }

func NewFilesService(dataDir string) *FilesService { return &FilesService{dataDir: dataDir} }

func (s *FilesService) SaveImage(key string, file multipart.File, removeExif bool) (string, error) {
	if !models.ValidKey(key) {
		return "", errors.New("invalid key format")
	}

	// ---- 1) 读入内存并限制大小（防止 OOM）----
	const maxUpload = int64(40 << 20) // 40MB
	lr := io.LimitReader(file, maxUpload+1)
	buf, err := io.ReadAll(lr)
	if err != nil {
		return "", err
	}
	if int64(len(buf)) > maxUpload {
		return "", errors.New("file too large")
	}

	// ---- 2) MIME 粗判 ----
	mediaType, err := detectMime(buf)
	if err != nil {
		return "", errors.New("unsupported file type: " + mediaType)
	}
	log.Println(mediaType)
	// ---- 3) 确保目录存在 ----
	dir := filepath.Join(s.dataDir, "entries", key, "images")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Println(err)
		return "", err
	}

	// ---- 4) 解码为 image.Image（HEIC/AVIF 单独走 libheif）----
	var img image.Image
	// 优先用 go-webp 解码（若输入本身是 WebP）
	if mediaType == "image/webp" || hasWebPMagic(buf) {
		img, err = webp.Decode(bytes.NewReader(buf), &decoder.Options{})
		// go-webp 用法参考官方示例。:contentReference[oaicite:1]{index=1}
	}
	// 回退到标准库（gif/jpeg/png）
	if img == nil && err == nil {
		img, _, err = image.Decode(bytes.NewReader(buf))
	}
	// （可选）再次回退：HEIC/HEIF/AVIF（需要系统已安装 libheif 及解码插件）
	if img == nil && err != nil && (mediaType == "image/heic" || mediaType == "image/heif" || mediaType == "image/avif") {
		img, err = DecodeFromBytes(buf)
	}

	if err != nil || img == nil {
		return "", err
	}

	// ---- 5) 统一编码为 WebP ----
	name := uuid.New().String() + ".webp"
	abs := filepath.Join(dir, name)
	dst, err := os.Create(abs)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	options, err := encoder.NewLossyEncoderOptions(encoder.PresetDefault, 75)
	if err != nil {
		return "", err
	}

	if err := webp.Encode(dst, img, options); err != nil {
		return "", err
	}

	return name, nil
}

// --- Helpers ---

func hasWebPMagic(b []byte) bool {
	// 简单判断 RIFF WEBP 头：RIFF....WEBP
	if len(b) < 12 {
		return false
	}
	return string(b[0:4]) == "RIFF" && string(b[8:12]) == "WEBP"
}

func DecodeFromBytes(buf []byte) (image.Image, error) {
	ctx, err := heif.NewContext()
	if err != nil {
		return nil, err
	}
	// 直接从内存读取，无需 io.Reader
	if err := ctx.ReadFromMemory(buf); err != nil {
		return nil, err
	}

	h, err := ctx.GetPrimaryImageHandle()
	if err != nil {
		return nil, err
	}

	// 解码到 RGB；如需 alpha，可用 ChromaInterleavedRGBA
	img, err := h.DecodeImage(heif.ColorspaceRGB, heif.ChromaInterleavedRGB, nil)
	if err != nil {
		return nil, err
	}

	// 转成 Go 的 image.Image
	return img.GetImage()
}
