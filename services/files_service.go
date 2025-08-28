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
	defer file.Close()

	const maxUpload = int64(40 << 20) // 40MB

	// 只读前 8KB 判断 MIME
	header := make([]byte, 8192)
	n, err := io.ReadFull(io.LimitReader(file, int64(len(header))), header)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", fmt.Errorf("read header failed: %w", err)
	}
	header = header[:n]

	mediaType, err := detectMime(header)
	if err != nil {
		return "", fmt.Errorf("unsupported file type: %w", err)
	}
	log.Printf("detected media type: %s", mediaType)

	// 确保目录存在
	dir := filepath.Join(s.dataDir, "entries", key, "images")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir failed: %w", err)
	}

	// 把整个文件读到内存（仍然限制大小）
	lr := io.LimitReader(io.MultiReader(bytes.NewReader(header), file), maxUpload+1)
	buf, err := io.ReadAll(lr)
	if err != nil {
		return "", fmt.Errorf("read file failed: %w", err)
	}
	if int64(len(buf)) > maxUpload {
		return "", errors.New("file too large")
	}

	// 解码图片
	img, err := decodeImage(buf, mediaType)
	if err != nil {
		return "", err
	}

	// 生成文件名
	name := uuid.New().String() + ".webp"
	abs := filepath.Join(dir, name)

	dst, err := os.Create(abs)
	if err != nil {
		return "", fmt.Errorf("create file failed: %w", err)
	}
	defer dst.Close()

	// webp 画质
	options, _ := encoder.NewLossyEncoderOptions(encoder.PresetPhoto, 75)
	if err := webp.Encode(dst, img, options); err != nil {
		return "", fmt.Errorf("webp encode failed: %w", err)
	}

	return name, nil
}

// decodeImage 根据类型选择解码器
func decodeImage(buf []byte, mediaType string) (image.Image, error) {
	switch mediaType {
	case "image/heic", "image/heif", "image/avif": //需要测试avif是否实际支持
		return DecodeFromBytes(buf)
	case "image/webp":
		return webp.Decode(bytes.NewReader(buf), &decoder.Options{})
	default:
		if hasWebPMagic(buf) {
			return webp.Decode(bytes.NewReader(buf), &decoder.Options{})
		}
		img, _, err := image.Decode(bytes.NewReader(buf))
		if err != nil {
			return nil, fmt.Errorf("std decode failed: %w", err)
		}
		return img, nil
	}
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
func detectMime(data []byte) (string, error) {
	mtype := mimetype.Detect(data)
	if !strings.HasPrefix(mtype.String(), "image/") {
		return "", fmt.Errorf("unsupported content type: %s", mtype.String())
	}
	return mtype.String(), nil
}
