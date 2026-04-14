package piper

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/wirvii/agent-speech/internal/config"
)

const (
	// DefaultModelDir es el directorio por defecto para los modelos piper.
	DefaultModelDir = "~/.local/share/agent-speech/models"
	// BaseURL es la URL base para descargar modelos de Hugging Face.
	BaseURL = "https://huggingface.co/rhasspy/piper-voices/resolve/v1.0.0"
)

// ModelInfo describe un modelo piper disponible.
type ModelInfo struct {
	ID      string // "es_MX-claude-high"
	Lang    string // "es"
	Region  string // "MX"
	Name    string // "claude"
	Quality string // "high"
}

// Catalog es el catalogo de modelos conocidos.
var Catalog = []ModelInfo{
	{ID: "es_MX-claude-high", Lang: "es", Region: "MX", Name: "claude", Quality: "high"},
	{ID: "es_MX-ald-medium", Lang: "es", Region: "MX", Name: "ald", Quality: "medium"},
	{ID: "en_US-lessac-medium", Lang: "en", Region: "US", Name: "lessac", Quality: "medium"},
}

// Resolve busca el modelo adecuado para un idioma y voz.
// Si voice esta especificado, busca por ID. Si no, retorna el primer modelo del idioma.
func Resolve(lang string, voice string) (*ModelInfo, error) {
	if voice != "" {
		for i := range Catalog {
			if Catalog[i].ID == voice {
				return &Catalog[i], nil
			}
		}
		return nil, fmt.Errorf("modelo %q no encontrado en el catalogo", voice)
	}

	// Idioma por defecto
	for i := range Catalog {
		if Catalog[i].Lang == lang {
			return &Catalog[i], nil
		}
	}

	return nil, fmt.Errorf("no hay modelos disponibles para el idioma %q", lang)
}

// ModelPath retorna la ruta al archivo .onnx del modelo.
// Error si no esta descargado.
func ModelPath(modelID string, modelDir string) (string, error) {
	dir, err := expandDir(modelDir)
	if err != nil {
		return "", fmt.Errorf("directorio de modelos: %w", err)
	}

	path := filepath.Join(dir, modelID+".onnx")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("modelo %q no descargado en %s", modelID, dir)
		}
		return "", err
	}
	return path, nil
}

// Download descarga el modelo .onnx y su .onnx.json al modelDir.
// Muestra progreso a stderr. Si ya existe, no descarga.
func Download(modelID string, modelDir string) error {
	dir, err := expandDir(modelDir)
	if err != nil {
		return fmt.Errorf("directorio de modelos: %w", err)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("crear directorio %s: %w", dir, err)
	}

	info, err := findInCatalog(modelID)
	if err != nil {
		return err
	}

	// Descargar .onnx y .onnx.json
	for _, suffix := range []string{".onnx", ".onnx.json"} {
		filename := modelID + suffix
		destPath := filepath.Join(dir, filename)

		if fileExists(destPath) {
			fmt.Fprintf(os.Stderr, "  %s ya existe, omitiendo\n", filename)
			continue
		}

		url := buildURL(info, filename)
		fmt.Fprintf(os.Stderr, "Descargando %s...\n", filename)

		if err := downloadFile(url, destPath); err != nil {
			return fmt.Errorf("descargar %s: %w", filename, err)
		}
		fmt.Fprintf(os.Stderr, "  Descargado: %s\n", destPath)
	}

	return nil
}

// buildURL construye la URL de descarga para un archivo de modelo.
// Patron: {BaseURL}/{lang}/{lang}_{region}/{name}/{quality}/{filename}
func buildURL(info *ModelInfo, filename string) string {
	lang := info.Lang
	langRegion := fmt.Sprintf("%s_%s", info.Lang, info.Region)
	return fmt.Sprintf("%s/%s/%s/%s/%s/%s",
		BaseURL, lang, langRegion, info.Name, info.Quality, filename)
}

// downloadFile descarga una URL a un archivo local con progreso.
func downloadFile(url, destPath string) error {
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return fmt.Errorf("HTTP GET: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d al descargar %s", resp.StatusCode, url)
	}

	tmp := destPath + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("crear archivo temporal: %w", err)
	}
	defer func() {
		f.Close()
		os.Remove(tmp)
	}()

	total := resp.ContentLength
	written, err := io.Copy(&progressWriter{dest: f, total: total}, resp.Body)
	if err != nil {
		return fmt.Errorf("escribir datos: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("cerrar archivo: %w", err)
	}

	if err := os.Rename(tmp, destPath); err != nil {
		return fmt.Errorf("mover archivo: %w", err)
	}

	_ = written
	return nil
}

// progressWriter escribe con progreso a stderr.
type progressWriter struct {
	dest    *os.File
	total   int64
	written int64
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.dest.Write(p)
	pw.written += int64(n)
	if pw.total > 0 {
		pct := float64(pw.written) / float64(pw.total) * 100
		size := formatBytes(pw.written)
		fmt.Fprintf(os.Stderr, "\r  %.1f%% (%s)", pct, size)
	}
	return n, err
}

// formatBytes formatea bytes en unidades legibles.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// findInCatalog busca un modelo en el catalogo por ID.
func findInCatalog(modelID string) (*ModelInfo, error) {
	for i := range Catalog {
		if Catalog[i].ID == modelID {
			return &Catalog[i], nil
		}
	}
	return nil, fmt.Errorf("modelo %q no encontrado en el catalogo.\n"+
		"  Modelos disponibles: %s", modelID, catalogIDs())
}

// catalogIDs retorna los IDs del catalogo como string.
func catalogIDs() string {
	ids := make([]string, len(Catalog))
	for i, m := range Catalog {
		ids[i] = m.ID
	}
	return strings.Join(ids, ", ")
}

// fileExists verifica si un archivo existe.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// expandDir expande ~ en el directorio.
func expandDir(dir string) (string, error) {
	return config.ExpandPath(dir)
}
