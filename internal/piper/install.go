package piper

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/wirvii/agent-speech/internal/config"
)

const (
	// PiperVersion es la version estable de piper a descargar.
	PiperVersion = "2023.11.14-2"

	// PiperReleaseBaseURL es la URL base de releases de piper en GitHub.
	PiperReleaseBaseURL = "https://github.com/rhasspy/piper/releases/download"

	// DefaultBinDir es el directorio donde se instala piper.
	DefaultBinDir = "~/.local/share/agent-speech/bin"
)

// archMap mapea runtime.GOARCH a los nombres de archivo de piper.
var archMap = map[string]string{
	"amd64": "piper_linux_x86_64",
	"arm64": "piper_linux_aarch64",
}

// BinPath retorna la ruta al binario piper interno.
// Retorna ("", false) si no esta instalado.
func BinPath() (string, bool) {
	dir, err := config.ExpandPath(DefaultBinDir)
	if err != nil {
		return "", false
	}
	path := filepath.Join(dir, "piper")
	if _, err := os.Stat(path); err != nil {
		return "", false
	}
	return path, true
}

// BinDir retorna la ruta expandida al directorio de binarios.
func BinDir() (string, error) {
	return config.ExpandPath(DefaultBinDir)
}

// Install descarga y extrae piper en DefaultBinDir.
// Muestra progreso a stderr. Si ya esta instalado, no hace nada.
// Retorna la ruta al binario instalado.
func Install() (string, error) {
	// Idempotente: si ya existe, no descargar
	if path, found := BinPath(); found {
		return path, nil
	}

	archName, ok := archMap[runtime.GOARCH]
	if !ok {
		return "", fmt.Errorf("arquitectura no soportada: %s (soportadas: amd64, arm64)", runtime.GOARCH)
	}

	url := fmt.Sprintf("%s/%s/%s.tar.gz", PiperReleaseBaseURL, PiperVersion, archName)

	dir, err := config.ExpandPath(DefaultBinDir)
	if err != nil {
		return "", fmt.Errorf("expandir directorio de binarios: %w", err)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("crear directorio %s: %w", dir, err)
	}

	tmpPath := filepath.Join(dir, ".piper-download.tar.gz")
	defer os.Remove(tmpPath) //nolint:errcheck

	fmt.Fprintf(os.Stderr, "  Descargando desde %s\n", url)
	if err := downloadFile(url, tmpPath); err != nil {
		return "", fmt.Errorf("descargar piper: %w", err)
	}
	fmt.Fprintln(os.Stderr)

	if err := extractTarGz(tmpPath, dir); err != nil {
		return "", fmt.Errorf("extraer piper: %w", err)
	}

	binPath := filepath.Join(dir, "piper")
	if _, err := os.Stat(binPath); err != nil {
		return "", fmt.Errorf("binario piper no encontrado tras extraccion en %s", dir)
	}

	return binPath, nil
}

// extractTarGz extrae un archivo .tar.gz stripeando el primer nivel de directorio.
func extractTarGz(tarPath, destDir string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return fmt.Errorf("abrir tar.gz: %w", err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("leer gzip: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("leer entry tar: %w", err)
		}

		// Seguridad: rechazar path traversal
		if strings.Contains(header.Name, "..") {
			return fmt.Errorf("path inseguro en tar: %s", header.Name)
		}

		// Stripear prefijo "piper/" del primer nivel
		name := header.Name
		if strings.HasPrefix(name, "piper/") {
			name = strings.TrimPrefix(name, "piper/")
		}
		// Entrada raiz "piper" o "piper/" — omitir
		if name == "" || name == "." {
			continue
		}

		destPath := filepath.Join(destDir, name)

		// Verificar que destPath este dentro de destDir (path traversal extra)
		if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("path fuera del directorio destino: %s", destPath)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destPath, 0o755); err != nil {
				return fmt.Errorf("crear directorio %s: %w", destPath, err)
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
				return fmt.Errorf("crear directorio padre de %s: %w", destPath, err)
			}

			// Determinar permisos: si tiene bits de ejecucion, usar 0o755
			perm := os.FileMode(0o644)
			if header.FileInfo().Mode()&0o111 != 0 {
				perm = 0o755
			}

			out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
			if err != nil {
				return fmt.Errorf("crear archivo %s: %w", destPath, err)
			}

			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return fmt.Errorf("escribir %s: %w", destPath, err)
			}
			out.Close()

			if err := os.Chmod(destPath, perm); err != nil {
				return fmt.Errorf("chmod %s: %w", destPath, err)
			}

		case tar.TypeSymlink:
			// Eliminar symlink previo si existe
			_ = os.Remove(destPath)
			if err := os.Symlink(header.Linkname, destPath); err != nil {
				return fmt.Errorf("crear symlink %s -> %s: %w", destPath, header.Linkname, err)
			}

		default:
			// Ignorar otros tipos (devices, etc.)
		}
	}

	return nil
}
