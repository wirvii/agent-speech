package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// githubAPIBase es la URL base de la API de GitHub. Es una variable para facilitar tests.
var githubAPIBase = "https://api.github.com"

// releaseResponse es la respuesta de la API de GitHub para un release.
type releaseResponse struct {
	TagName string          `json:"tag_name"`
	Assets  []releaseAsset  `json:"assets"`
}

// releaseAsset es un asset de un release de GitHub.
type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// CheckLatest consulta la GitHub API y retorna la ultima version disponible.
// repo debe ser "owner/repo", ej: "wirvii/agent-speech".
func CheckLatest(repo string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", githubAPIBase, repo)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("crear request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "agent-speech-updater")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("consultar GitHub API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API retorno status %d", resp.StatusCode)
	}

	var release releaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("parsear respuesta de GitHub: %w", err)
	}

	if release.TagName == "" {
		return "", fmt.Errorf("no se encontro tag_name en la respuesta de GitHub")
	}

	return release.TagName, nil
}

// NeedsUpdate compara la version actual con la ultima y retorna true si hay actualizacion.
// Las versiones deben ser del tipo "v1.2.3". Si las versiones son iguales retorna false.
// Si current es "dev" siempre retorna true (build de desarrollo).
func NeedsUpdate(current, latest string) bool {
	if current == "dev" || current == "" {
		return true
	}
	// Normalizar: remover prefijo "v" para comparar
	c := strings.TrimPrefix(current, "v")
	l := strings.TrimPrefix(latest, "v")
	return c != l
}

// Update descarga el binario para la plataforma actual y reemplaza el ejecutable.
// repo: "wirvii/agent-speech"
// version: "v0.3.0" (tag del release)
func Update(repo, version string) error {
	// Determinar el asset correcto para esta plataforma
	assetName := fmt.Sprintf("agent-speech-%s-%s", runtime.GOOS, runtime.GOARCH)

	// Obtener la URL de descarga del asset
	downloadURL, err := resolveDownloadURL(repo, version, assetName)
	if err != nil {
		return fmt.Errorf("resolver URL de descarga: %w", err)
	}

	// Obtener ruta del ejecutable actual
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("obtener ruta del ejecutable: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolver symlinks del ejecutable: %w", err)
	}

	execDir := filepath.Dir(execPath)

	// Verificar permisos de escritura en el directorio del ejecutable
	if err := checkWritePermission(execDir); err != nil {
		return fmt.Errorf("sin permisos de escritura en %s: %w\n  Intenta ejecutar con sudo o mover el binario a un directorio con permisos", execDir, err)
	}

	// Descargar a archivo temporal en el mismo directorio (mismo filesystem -> rename atomico)
	tmpFile, err := os.CreateTemp(execDir, "agent-speech-update-*")
	if err != nil {
		return fmt.Errorf("crear archivo temporal en %s: %w", execDir, err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		// Limpiar el temporal si algo falla (rename exitoso lo mueve, el defer es no-op)
		os.Remove(tmpPath) //nolint:errcheck
	}()

	// Descargar el binario
	if err := downloadBinary(downloadURL, tmpFile); err != nil {
		tmpFile.Close()
		return fmt.Errorf("descargar binario: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("cerrar archivo temporal: %w", err)
	}

	// Dar permisos de ejecucion
	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("establecer permisos del binario: %w", err)
	}

	// Reemplazar atomicamente el ejecutable actual
	if err := os.Rename(tmpPath, execPath); err != nil {
		return fmt.Errorf("reemplazar ejecutable: %w", err)
	}

	return nil
}

// resolveDownloadURL obtiene la URL de descarga de un asset especifico de un release.
func resolveDownloadURL(repo, version, assetName string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/tags/%s", githubAPIBase, repo, version)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("crear request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "agent-speech-updater")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("consultar GitHub API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("release %s no encontrado en %s", version, repo)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API retorno status %d", resp.StatusCode)
	}

	var release releaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("parsear respuesta de GitHub: %w", err)
	}

	for _, asset := range release.Assets {
		if asset.Name == assetName {
			return asset.BrowserDownloadURL, nil
		}
	}

	return "", fmt.Errorf("asset %q no encontrado en release %s (plataforma: %s/%s)", assetName, version, runtime.GOOS, runtime.GOARCH)
}

// downloadBinary descarga el binario desde url y lo escribe en dst.
func downloadBinary(url string, dst io.Writer) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("crear request: %w", err)
	}
	req.Header.Set("User-Agent", "agent-speech-updater")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("descargar desde %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("descarga retorno status %d", resp.StatusCode)
	}

	if _, err := io.Copy(dst, resp.Body); err != nil {
		return fmt.Errorf("escribir binario: %w", err)
	}

	return nil
}

// checkWritePermission verifica si se puede escribir en el directorio dado.
func checkWritePermission(dir string) error {
	tmp, err := os.CreateTemp(dir, ".write-check-*")
	if err != nil {
		return err
	}
	tmp.Close()            //nolint:errcheck
	os.Remove(tmp.Name())  //nolint:errcheck
	return nil
}
