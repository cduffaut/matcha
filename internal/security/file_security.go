// internal/security/file_security.go
package security

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rwcarlsen/goexif/exif"
)

/* ===== config ===== */

const (
	MaxFileSize    = 8 * 1024 * 1024
	MaxImageWidth  = 5000
	MaxImageHeight = 5000
	MinImageWidth  = 50
	MinImageHeight = 50
)

var (
	allowedMimes = map[string]struct{}{
		"image/jpeg": {}, "image/jpg": {}, "image/png": {}, "image/gif": {},
	}
	allowedExts = map[string]struct{}{
		".jpg": {}, ".jpeg": {}, ".png": {}, ".gif": {},
	}
	filenameBadChars = regexp.MustCompile(`[^A-Za-z0-9._-]`)
)

/* ===== types ===== */

type FileValidationError struct{ Message string }

func (e FileValidationError) Error() string { return e.Message }

/* ===== public API ===== */

func ValidateImageUpload(h *multipart.FileHeader, data []byte) error {
	if len(data) == 0 {
		return FileValidationError{"le fichier est vide"}
	}
	if len(data) > MaxFileSize {
		return FileValidationError{fmt.Sprintf("le fichier est trop volumineux (max %d MB)", MaxFileSize/1024/1024)}
	}

	ext := strings.ToLower(filepath.Ext(h.Filename))
	if _, ok := allowedExts[ext]; !ok {
		return FileValidationError{"type de fichier non autorisé (JPG/PNG/GIF)"}
	}

	decl := normalizeMime(h.Header.Get("Content-Type"))
	if _, ok := allowedMimes[decl]; !ok {
		return FileValidationError{"type MIME non autorisé"}
	}

	detected, err := detectImageType(data)
	if err != nil {
		return FileValidationError{"impossible de détecter le type de fichier"}
	}
	if normalizeMime(decl) != normalizeMime(detected) {
		return FileValidationError{"le type déclaré ne correspond pas au contenu"}
	}

	if err := validateImageDimensions(data); err != nil {
		return err
	}
	if err := scanForMaliciousContent(data); err != nil {
		return err
	}
	return nil
}

func SanitizeFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." {
		return "file"
	}
	ext := strings.ToLower(filepath.Ext(name))
	base := strings.TrimSuffix(name, ext)
	base = filenameBadChars.ReplaceAllString(base, "_")
	if len(base) > 96 {
		base = base[:96]
	}
	if ext == "" {
		ext = ".bin"
	}
	return base + ext
}

func ProcessAndValidateImage(h *multipart.FileHeader, data []byte) ([]byte, error) {
	if err := ValidateImageUpload(h, data); err != nil {
		return nil, err
	}
	out, err := reencodeImage(data)
	if err != nil {
		return nil, FileValidationError{"erreur lors du traitement de l'image"}
	}
	return out, nil
}

/* ===== internals ===== */

func normalizeMime(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	if t == "image/jpg" {
		return "image/jpeg"
	}
	return t
}

func detectImageType(b []byte) (string, error) {
	// quick magic-bytes
	switch {
	case len(b) >= 8 && bytes.HasPrefix(b, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}):
		return "image/png", nil
	case len(b) >= 3 && bytes.HasPrefix(b, []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg", nil
	case len(b) >= 6 && (bytes.HasPrefix(b, []byte("GIF87a")) || bytes.HasPrefix(b, []byte("GIF89a"))):
		return "image/gif", nil
	}
	// fallback
	return http.DetectContentType(b), nil
}

func scanForMaliciousContent(b []byte) error {
	// inspect small header window only
	n := 1024
	if len(b) < n {
		n = len(b)
	}
	s := strings.ToLower(string(b[:n]))
	danger := []string{"<?php", "<script", "javascript:", "vbscript:", "<iframe", "<object", "<embed", "<%"}
	for _, d := range danger {
		if i := strings.Index(s, d); i >= 0 && looksLikeScript(s[i:]) {
			return FileValidationError{"contenu script détecté dans l'image"}
		}
	}
	// ensure decoded format is supported
	if _, format, err := image.DecodeConfig(bytes.NewReader(b)); err != nil {
		return FileValidationError{"fichier image invalide ou corrompu"}
	} else if format != "jpeg" && format != "png" && format != "gif" {
		return FileValidationError{"format d'image non supporté"}
	}
	return nil
}

func looksLikeScript(ctx string) bool {
	if len(ctx) > 64 {
		ctx = ctx[:64]
	}
	for _, p := range []string{">", "=", "(", "{", "src="} {
		if strings.Contains(ctx, p) {
			return true
		}
	}
	return false
}

func validateImageDimensions(b []byte) error {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return FileValidationError{"impossible de lire les propriétés de l'image"}
	}
	if cfg.Width > MaxImageWidth || cfg.Height > MaxImageHeight {
		return FileValidationError{fmt.Sprintf(
			"image trop grande (%dx%d). Maximum %dx%d pixels",
			cfg.Width, cfg.Height, MaxImageWidth, MaxImageHeight)}
	}
	if cfg.Width < MinImageWidth || cfg.Height < MinImageHeight {
		return FileValidationError{fmt.Sprintf(
			"image trop petite (%dx%d). Minimum %dx%d pixels",
			cfg.Width, cfg.Height, MinImageWidth, MinImageHeight)}
	}
	return nil
}

func reencodeImage(b []byte) ([]byte, error) {
	img, format, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	if oriented, err := applyEXIFOrientation(b, img); err == nil {
		img = oriented
	}
	var buf bytes.Buffer
	switch format {
	case "jpeg":
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85})
	case "png":
		err = png.Encode(&buf, img)
	case "gif":
		err = gif.Encode(&buf, img, nil)
	default:
		return nil, fmt.Errorf("format non supporté: %s", format)
	}
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func applyEXIFOrientation(src []byte, img image.Image) (image.Image, error) {
	x, err := exif.Decode(bytes.NewReader(src))
	if err != nil {
		return img, nil
	}
	tag, err := x.Get(exif.Orientation)
	if err != nil {
		return img, nil
	}
	v, err := tag.Int(0)
	if err != nil {
		return img, nil
	}
	switch v {
	case 2:
		return flipHorizontal(img), nil
	case 3:
		return rotate180(img), nil
	case 4:
		return flipVertical(img), nil
	case 5:
		return rotate90Clockwise(flipHorizontal(img)), nil
	case 6:
		return rotate90Clockwise(img), nil
	case 7:
		return rotate90CounterClockwise(flipHorizontal(img)), nil
	case 8:
		return rotate90CounterClockwise(img), nil
	default:
		return img, nil
	}
}

/* ===== transforms ===== */

func rotate90Clockwise(img image.Image) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := image.NewRGBA(image.Rect(0, 0, h, w))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			out.Set(h-1-(y-b.Min.Y), x-b.Min.X, img.At(x, y))
		}
	}
	return out
}

func rotate90CounterClockwise(img image.Image) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := image.NewRGBA(image.Rect(0, 0, h, w))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			out.Set(y-b.Min.Y, w-1-(x-b.Min.X), img.At(x, y))
		}
	}
	return out
}

func rotate180(img image.Image) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			out.Set(w-1-(x-b.Min.X), h-1-(y-b.Min.Y), img.At(x, y))
		}
	}
	return out
}

func flipHorizontal(img image.Image) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			out.Set(w-1-(x-b.Min.X), y-b.Min.Y, img.At(x, y))
		}
	}
	return out
}

func flipVertical(img image.Image) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			out.Set(x-b.Min.X, h-1-(y-b.Min.Y), img.At(x, y))
		}
	}
	return out
}
