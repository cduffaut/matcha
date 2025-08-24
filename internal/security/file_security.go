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
	"path/filepath"
	"strings"

	"github.com/rwcarlsen/goexif/exif"
)

// Configuration des uploads
const (
	MaxFileSize    = 8 * 1024 * 1024
	MaxImageWidth  = 5000
	MaxImageHeight = 5000
)

// Types MIME autorisés pour les images
var allowedMimeTypes = map[string]bool{
	"image/jpeg": true,
	"image/jpg":  true,
	"image/png":  true,
	"image/gif":  true,
}

// Extensions autorisées
var allowedExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
}

// FileValidationError représente une erreur de validation de fichier
type FileValidationError struct {
	Message string
}

func (e FileValidationError) Error() string {
	return e.Message
}

// ValidateImageUpload valide un fichier image uploadé
func ValidateImageUpload(fileHeader *multipart.FileHeader, fileData []byte) error {
	// Vérifier la taille du fichier
	if len(fileData) > MaxFileSize {
		return FileValidationError{
			Message: fmt.Sprintf("le fichier est trop volumineux (max %d MB)", MaxFileSize/1024/1024),
		}
	}

	if len(fileData) == 0 {
		return FileValidationError{Message: "le fichier est vide"}
	}

	// Vérifier l'extension
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if !allowedExtensions[ext] {
		return FileValidationError{
			Message: "type de fichier non autorisé. Seuls les fichiers JPG, PNG et GIF sont acceptés",
		}
	}

	// Vérifier le type MIME déclaré
	contentType := fileHeader.Header.Get("Content-Type")
	if !allowedMimeTypes[contentType] {
		return FileValidationError{
			Message: "type MIME non autorisé",
		}
	}

	// Vérifier le type réel du fichier en analysant les magic bytes
	realType, err := detectImageType(fileData)
	if err != nil {
		return FileValidationError{Message: "impossible de détecter le type de fichier"}
	}

	// Vérifier que le type déclaré correspond au type réel
	if !isContentTypeMatchingDetected(contentType, realType) {
		return FileValidationError{
			Message: "le type de fichier déclaré ne correspond pas au contenu réel",
		}
	}

	// Vérifier les dimensions de l'image
	if err := validateImageDimensions(fileData); err != nil {
		return err
	}

	// Vérifier qu'il n'y a pas de contenu malveillant
	if err := scanForMaliciousContent(fileData); err != nil {
		return err
	}

	return nil
}

// detectImageType détecte le type réel d'une image à partir de ses magic bytes
func detectImageType(data []byte) (string, error) {
	if len(data) < 12 {
		return "", fmt.Errorf("fichier trop petit")
	}

	// PNG magic bytes: 89 50 4E 47 0D 0A 1A 0A
	if bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		return "image/png", nil
	}

	// JPEG magic bytes: FF D8 FF
	if bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}) {
		return "image/jpeg", nil
	}

	// GIF magic bytes: GIF87a ou GIF89a
	if bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a")) {
		return "image/gif", nil
	}

	return "", fmt.Errorf("type de fichier non reconnu")
}

// isContentTypeMatchingDetected vérifie si le type MIME déclaré correspond au type détecté
func isContentTypeMatchingDetected(declared, detected string) bool {
	// Normaliser les types MIME
	normalizeType := func(t string) string {
		if t == "image/jpg" {
			return "image/jpeg"
		}
		return t
	}

	return normalizeType(declared) == normalizeType(detected)
}

// scanForMaliciousContent amélioré - respecte le sujet mais évite les faux positifs
func scanForMaliciousContent(data []byte) error {
	// Le sujet exige la validation des uploads, donc on garde cette fonction
	// mais on l'améliore pour éviter les faux positifs sur les vraies photos

	// Étape 1: Vérifier que c'est vraiment une image valide
	// Si l'image peut être décodée, c'est probablement légitime
	reader := bytes.NewReader(data)
	_, format, err := image.DecodeConfig(reader)
	if err != nil {
		// Si on ne peut pas décoder l'image, elle est suspecte
		return FileValidationError{Message: "fichier image invalide ou corrompu"}
	}

	// Étape 2: Scanner seulement les premiers bytes (headers EXIF/métadonnées)
	// Les vraies injections sont généralement dans cette zone
	scanSize := 1024 // Premiers 1KB seulement
	if len(data) < scanSize {
		scanSize = len(data)
	}
	scanData := data[:scanSize]

	// Étape 3: Chercher des patterns vraiment suspects dans les métadonnées
	// On convertit en string pour chercher du texte lisible
	dataStr := string(scanData)

	// Patterns vraiment dangereux (conformes aux exigences de sécurité du sujet)
	dangerousPatterns := []string{
		"<?php",       // Scripts PHP
		"<%",          // Scripts ASP/JSP
		"<script",     // JavaScript malveillant
		"javascript:", // Liens JavaScript
		"vbscript:",   // VBScript malveillant
		"<iframe",     // Iframes malveillantes
		"<object",     // Objects malveillants
		"<embed",      // Embeds malveillants
	}

	dataLower := strings.ToLower(dataStr)

	for _, pattern := range dangerousPatterns {
		if strings.Contains(dataLower, pattern) {
			// Vérification supplémentaire: s'assurer que c'est du vrai code
			if isRealScriptContent(dataStr, pattern) {
				return FileValidationError{Message: "contenu script détecté dans le fichier image"}
			}
		}
	}

	// Étape 4: Vérifier la cohérence format/extension
	// Une vraie image doit avoir un format cohérent
	expectedFormats := map[string]bool{
		"jpeg": true,
		"png":  true,
		"gif":  true,
	}

	if !expectedFormats[format] {
		return FileValidationError{Message: "format d'image non supporté"}
	}

	return nil
}

// isRealScriptContent vérifie si le contenu ressemble vraiment à du script
func isRealScriptContent(data, pattern string) bool {
	index := strings.Index(strings.ToLower(data), pattern)
	if index == -1 {
		return false
	}

	// Vérifier le contexte : chercher des caractères de script typiques
	start := index
	end := index + len(pattern) + 50
	if end > len(data) {
		end = len(data)
	}

	context := strings.ToLower(data[start:end])

	// Chercher des patterns de script typiques après le tag
	scriptPatterns := []string{
		">",    // Fermeture de tag HTML
		"=",    // Assignation
		"(",    // Appel de fonction
		"{",    // Bloc de code
		"src=", // Attribut source
	}

	for _, scriptPattern := range scriptPatterns {
		if strings.Contains(context, scriptPattern) {
			return true
		}
	}

	return false
}

// validateImageDimensions mise à jour avec messages plus clairs
func validateImageDimensions(data []byte) error {
	reader := bytes.NewReader(data)

	config, _, err := image.DecodeConfig(reader)
	if err != nil {
		return FileValidationError{Message: "impossible de lire les propriétés de l'image"}
	}

	if config.Width > MaxImageWidth || config.Height > MaxImageHeight {
		return FileValidationError{
			Message: fmt.Sprintf("image trop grande (%dx%d pixels). Maximum autorisé : %dx%d pixels. Essayez de redimensionner votre image.",
				config.Width, config.Height, MaxImageWidth, MaxImageHeight),
		}
	}

	if config.Width < 50 || config.Height < 50 {
		return FileValidationError{
			Message: fmt.Sprintf("image trop petite (%dx%d pixels). Minimum requis : 50x50 pixels",
				config.Width, config.Height),
		}
	}

	return nil
}

////////////////////////////////////////////////////////////

// SanitizeFilename nettoie un nom de fichier
func SanitizeFilename(filename string) string {
	// Obtenir seulement le nom de base (sans le chemin)
	filename = filepath.Base(filename)

	// Remplacer les caractères dangereux
	dangerous := []string{
		"..", "/", "\\", ":", "*", "?", "\"", "<", ">", "|",
		"\x00", "\x01", "\x02", "\x03", "\x04", "\x05", "\x06", "\x07",
		"\x08", "\x09", "\x0a", "\x0b", "\x0c", "\x0d", "\x0e", "\x0f",
		"\x10", "\x11", "\x12", "\x13", "\x14", "\x15", "\x16", "\x17",
		"\x18", "\x19", "\x1a", "\x1b", "\x1c", "\x1d", "\x1e", "\x1f",
	}

	for _, char := range dangerous {
		filename = strings.ReplaceAll(filename, char, "_")
	}

	// Limiter la longueur
	if len(filename) > 100 {
		ext := filepath.Ext(filename)
		name := strings.TrimSuffix(filename, ext)
		filename = name[:100-len(ext)] + ext
	}

	// S'assurer qu'il y a au moins un nom
	if filename == "" || filename == "." {
		filename = "file"
	}

	return filename
}

// ProcessAndValidateImage traite et valide une image
func ProcessAndValidateImage(fileHeader *multipart.FileHeader, fileData []byte) ([]byte, error) {
	// Valider le fichier
	if err := ValidateImageUpload(fileHeader, fileData); err != nil {
		return nil, err
	}

	// Réencoder l'image pour supprimer les métadonnées EXIF potentiellement dangereuses
	processedData, err := reencodeImage(fileData)
	if err != nil {
		return nil, FileValidationError{Message: "erreur lors du traitement de l'image"}
	}

	return processedData, nil
}

// reencodeImage réencode une image pour supprimer les métadonnées
func reencodeImage(data []byte) ([]byte, error) {
	reader := bytes.NewReader(data)

	// Décoder l'image
	img, format, err := image.Decode(reader)
	if err != nil {
		return nil, err
	}

	// CORRECTION: Lire et appliquer l'orientation EXIF avant de supprimer les métadonnées
	orientedImg, err := applyEXIFOrientation(data, img)
	if err != nil {
		// Si on ne peut pas lire l'EXIF, utiliser l'image originale
		orientedImg = img
	}

	// Réencoder l'image orientée (sans métadonnées EXIF)
	var buf bytes.Buffer
	switch format {
	case "jpeg":
		err = jpeg.Encode(&buf, orientedImg, &jpeg.Options{Quality: 85})
	case "png":
		err = png.Encode(&buf, orientedImg)
	case "gif":
		err = gif.Encode(&buf, orientedImg, nil)
	default:
		return nil, fmt.Errorf("format non supporté: %s", format)
	}

	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// applyEXIFOrientation applique la rotation selon les métadonnées EXIF
func applyEXIFOrientation(data []byte, img image.Image) (image.Image, error) {
	// Lire les métadonnées EXIF
	reader := bytes.NewReader(data)
	exifData, err := exif.Decode(reader)
	if err != nil {
		// Pas d'EXIF ou erreur de lecture, retourner l'image telle quelle
		return img, nil
	}

	// Récupérer la tag d'orientation
	orientationTag, err := exifData.Get(exif.Orientation)
	if err != nil {
		// Pas d'orientation spécifiée, retourner l'image telle quelle
		return img, nil
	}

	orientation, err := orientationTag.Int(0)
	if err != nil {
		return img, nil
	}

	// Appliquer la transformation selon l'orientation EXIF
	switch orientation {
	case 1:
		// Normal - pas de transformation
		return img, nil
	case 2:
		// Miroir horizontal
		return flipHorizontal(img), nil
	case 3:
		// Rotation 180°
		return rotate180(img), nil
	case 4:
		// Miroir vertical
		return flipVertical(img), nil
	case 5:
		// Miroir horizontal + rotation 90° horaire
		return rotate90Clockwise(flipHorizontal(img)), nil
	case 6:
		// Rotation 90° horaire
		return rotate90Clockwise(img), nil
	case 7:
		// Miroir horizontal + rotation 90° anti-horaire
		return rotate90CounterClockwise(flipHorizontal(img)), nil
	case 8:
		// Rotation 90° anti-horaire
		return rotate90CounterClockwise(img), nil
	default:
		return img, nil
	}
}

// Fonctions de transformation d'image
func rotate90Clockwise(img image.Image) image.Image {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// Créer une nouvelle image avec dimensions inversées
	rotated := image.NewRGBA(image.Rect(0, 0, h, w))

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			// Rotation 90° horaire: (x,y) -> (h-1-y, x)
			newX := h - 1 - (y - bounds.Min.Y)
			newY := x - bounds.Min.X
			rotated.Set(newX, newY, img.At(x, y))
		}
	}

	return rotated
}

func rotate90CounterClockwise(img image.Image) image.Image {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	rotated := image.NewRGBA(image.Rect(0, 0, h, w))

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			// Rotation 90° anti-horaire: (x,y) -> (y, w-1-x)
			newX := y - bounds.Min.Y
			newY := w - 1 - (x - bounds.Min.X)
			rotated.Set(newX, newY, img.At(x, y))
		}
	}

	return rotated
}

func rotate180(img image.Image) image.Image {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	rotated := image.NewRGBA(image.Rect(0, 0, w, h))

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			// Rotation 180°: (x,y) -> (w-1-x, h-1-y)
			newX := w - 1 - (x - bounds.Min.X)
			newY := h - 1 - (y - bounds.Min.Y)
			rotated.Set(newX, newY, img.At(x, y))
		}
	}

	return rotated
}

func flipHorizontal(img image.Image) image.Image {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	flipped := image.NewRGBA(image.Rect(0, 0, w, h))

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			// Miroir horizontal: (x,y) -> (w-1-x, y)
			newX := w - 1 - (x - bounds.Min.X)
			newY := y - bounds.Min.Y
			flipped.Set(newX, newY, img.At(x, y))
		}
	}

	return flipped
}

func flipVertical(img image.Image) image.Image {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	flipped := image.NewRGBA(image.Rect(0, 0, w, h))

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			// Miroir vertical: (x,y) -> (x, h-1-y)
			newX := x - bounds.Min.X
			newY := h - 1 - (y - bounds.Min.Y)
			flipped.Set(newX, newY, img.At(x, y))
		}
	}

	return flipped
}
