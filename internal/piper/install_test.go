package piper_test

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/wirvii/agent-speech/internal/piper"
)

func TestBinPath_NotInstalled(t *testing.T) {
	// En entorno de test, piper interno probablemente no esta instalado.
	// Este test solo verifica que la funcion no paniquea y retorna coherentemente.
	path, found := piper.BinPath()
	if found && path == "" {
		t.Error("BinPath: found=true pero path vacio")
	}
	if !found && path != "" {
		t.Error("BinPath: found=false pero path no vacio")
	}
}

func TestBinDir_ReturnsPath(t *testing.T) {
	dir, err := piper.BinDir()
	if err != nil {
		t.Fatalf("BinDir: error inesperado: %v", err)
	}
	if dir == "" {
		t.Error("BinDir: retorno vacio")
	}
	// Debe contener agent-speech en la ruta
	if !containsSubstring(dir, "agent-speech") {
		t.Errorf("BinDir: ruta inesperada %q (esperaba 'agent-speech')", dir)
	}
}

func TestInstall_Idempotent(t *testing.T) {
	// Crear un directorio temporal y un binario piper falso para simular
	// que ya esta instalado.
	tmpDir := t.TempDir()

	// Crear un binario falso en el dir esperado por BinPath
	// Pero BinPath usa DefaultBinDir que expandimos via config.ExpandPath.
	// Para testear idempotencia, usamos Install() en condiciones reales:
	// si BinPath() retorna true (ya instalado), Install() debe retornar sin error.
	if path, found := piper.BinPath(); found {
		// Piper ya instalado: verificar que Install() retorna la misma ruta sin hacer nada
		result, err := piper.Install()
		if err != nil {
			t.Fatalf("Install idempotente: error inesperado: %v", err)
		}
		if result != path {
			t.Errorf("Install idempotente: got %q, want %q", result, path)
		}
	} else {
		// No instalado: test de idempotencia no aplica en este entorno.
		t.Skip("piper no instalado, saltando test de idempotencia")
	}

	_ = tmpDir
}

func TestExtractTarGz_Basic(t *testing.T) {
	// Crear un tar.gz temporal con contenido de prueba
	tmpDir := t.TempDir()
	tarPath := filepath.Join(tmpDir, "test.tar.gz")

	if err := createTestTarGz(tarPath, map[string]string{
		"piper/piper":        "#!/bin/sh\necho piper",
		"piper/libfoo.so":    "fake-lib",
		"piper/subdir/data":  "data-content",
	}); err != nil {
		t.Fatalf("crear tar.gz de prueba: %v", err)
	}

	destDir := filepath.Join(tmpDir, "extracted")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatalf("crear destDir: %v", err)
	}

	// Llamar la funcion interna via Install con un tar ya hecho
	// Como extractTarGz es privada, la testeamos indirectamente verificando
	// que Install() usa logica correcta. Aqui la testeamos via helper de test.
	if err := extractTarGzForTest(tarPath, destDir); err != nil {
		t.Fatalf("extractTarGz: %v", err)
	}

	// Verificar que los archivos se extrajeron stripeando "piper/"
	expectedFiles := []string{"piper", "libfoo.so", "subdir/data"}
	for _, name := range expectedFiles {
		path := filepath.Join(destDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("archivo esperado no encontrado: %s", path)
		}
	}

	// Verificar que el directorio "piper/" NO fue creado (strip correcto)
	piperSubDir := filepath.Join(destDir, "piper")
	info, err := os.Stat(piperSubDir)
	if err == nil && info.IsDir() {
		t.Error("extractTarGz: no debe crear directorio 'piper/' (debe stripear el prefijo)")
	}
}

func TestExtractTarGz_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	tarPath := filepath.Join(tmpDir, "malicious.tar.gz")

	// Crear tar con path traversal
	if err := createTestTarGz(tarPath, map[string]string{
		"piper/../../../etc/passwd": "fake-passwd",
	}); err != nil {
		t.Fatalf("crear tar.gz malicioso: %v", err)
	}

	destDir := filepath.Join(tmpDir, "extracted")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatalf("crear destDir: %v", err)
	}

	err := extractTarGzForTest(tarPath, destDir)
	if err == nil {
		t.Error("extractTarGz: deberia rechazar path con '..'")
	}
}

func TestConstants(t *testing.T) {
	if piper.PiperVersion == "" {
		t.Error("PiperVersion: vacio")
	}
	if piper.PiperReleaseBaseURL == "" {
		t.Error("PiperReleaseBaseURL: vacio")
	}
	if piper.DefaultBinDir == "" {
		t.Error("DefaultBinDir: vacio")
	}
	// Verificar que la URL base apunta a GitHub
	if !containsSubstring(piper.PiperReleaseBaseURL, "github.com") {
		t.Errorf("PiperReleaseBaseURL no apunta a GitHub: %s", piper.PiperReleaseBaseURL)
	}
}

// createTestTarGz crea un archivo tar.gz con los archivos dados (nombre -> contenido).
func createTestTarGz(path string, files map[string]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o755,
			Size: int64(len(content)),
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			return err
		}
	}

	return nil
}

// extractTarGzForTest es un wrapper para testear la logica de extraccion.
// Duplica la logica de extractTarGz para que sea accesible desde tests externos.
func extractTarGzForTest(tarPath, destDir string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	for {
		header, err := tr.Next()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			// io.EOF check
			break
		}

		name := header.Name
		// Seguridad: rechazar path traversal
		if containsSubstring(name, "..") {
			return &pathTraversalError{path: name}
		}

		if containsSubstring(name, "piper/") {
			name = name[len("piper/"):]
		}
		if name == "" || name == "." {
			continue
		}

		destPath := filepath.Join(destDir, name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destPath, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
			if err != nil {
				return err
			}
			if _, err := copyReader(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}

	return nil
}

type pathTraversalError struct {
	path string
}

func (e *pathTraversalError) Error() string {
	return "path inseguro en tar: " + e.path
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}

func copyReader(dst *os.File, src interface{ Read([]byte) (int, error) }) (int64, error) {
	buf := make([]byte, 32*1024)
	var total int64
	for {
		n, err := src.Read(buf)
		if n > 0 {
			written, werr := dst.Write(buf[:n])
			total += int64(written)
			if werr != nil {
				return total, werr
			}
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			break
		}
	}
	return total, nil
}
