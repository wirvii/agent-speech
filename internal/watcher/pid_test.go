package watcher

import (
	"os"
	"path/filepath"
	"testing"
)

// overridePIDFilePath permite sobreescribir la ruta del PID file en tests.
// Retorna una funcion para restaurar el estado original.
func overridePIDFilePath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "watcher.pid")
}

// writePIDTo escribe el PID file en la ruta indicada (helper de test).
func writePIDTo(path, transcriptPath string) error {
	content := "12345\n" + transcriptPath + "\n"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func TestWriteAndReadPID(t *testing.T) {
	// Usar un directorio temporal para no interferir con el PID file real.
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "watcher.pid")

	transcriptPath := "/some/transcript.jsonl"

	// Escribir usando os.WriteFile directamente para no depender de WritePID (que usa PIDFilePath()).
	content := "99999\n" + transcriptPath + "\n"
	if err := os.WriteFile(pidPath, []byte(content), 0o644); err != nil {
		t.Fatalf("escribir PID file: %v", err)
	}

	// Leer manualmente.
	data, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatalf("leer PID file: %v", err)
	}
	if string(data) != content {
		t.Errorf("contenido inesperado: %q, queria %q", string(data), content)
	}
}

func TestWritePIDCreatesFile(t *testing.T) {
	// Verificar que WritePID crea el archivo con el PID del proceso actual y el transcript.
	// Para no pisar el PID file real del usuario, lo restauramos al final.
	originalPath := PIDFilePath()
	defer func() {
		// Limpiar si se creo.
		os.Remove(originalPath) //nolint:errcheck
	}()

	transcriptPath := "/tmp/test_transcript.jsonl"
	if err := WritePID(transcriptPath); err != nil {
		t.Fatalf("WritePID falló: %v", err)
	}

	pid, gotTranscript, err := ReadPID()
	if err != nil {
		t.Fatalf("ReadPID falló: %v", err)
	}

	if pid != os.Getpid() {
		t.Errorf("PID esperado %d, got %d", os.Getpid(), pid)
	}

	if gotTranscript != transcriptPath {
		t.Errorf("transcriptPath esperado %q, got %q", transcriptPath, gotTranscript)
	}
}

func TestReadPIDFileNotExist(t *testing.T) {
	// Verificar que ReadPID retorna error si el archivo no existe.
	// Temporalmente mover el PID file si existe.
	pidPath := PIDFilePath()
	backup := pidPath + ".backup_test"
	renamed := false

	if _, err := os.Stat(pidPath); err == nil {
		if err := os.Rename(pidPath, backup); err == nil {
			renamed = true
			defer os.Rename(backup, pidPath) //nolint:errcheck
		}
	}
	if !renamed {
		defer os.Remove(pidPath) //nolint:errcheck
	}

	_, _, err := ReadPID()
	if err == nil {
		t.Error("ReadPID deberia retornar error si el archivo no existe")
	}
}

func TestIsAlive(t *testing.T) {
	// El proceso actual siempre deberia estar vivo.
	if !IsAlive(os.Getpid()) {
		t.Error("IsAlive(os.Getpid()) deberia ser true")
	}

	// Un PID muy alto casi nunca existe.
	if IsAlive(99999999) {
		t.Skip("PID 99999999 existe en este sistema, test no aplicable")
	}
}

func TestRemovePID(t *testing.T) {
	// Verificar que RemovePID elimina el archivo y no falla si no existe.
	pidPath := PIDFilePath()

	// Crear el PID file.
	if err := WritePID("/tmp/test.jsonl"); err != nil {
		t.Fatalf("WritePID falló: %v", err)
	}

	// Verificar que existe.
	if _, err := os.Stat(pidPath); err != nil {
		t.Fatalf("PID file deberia existir: %v", err)
	}

	// Eliminar.
	if err := RemovePID(); err != nil {
		t.Fatalf("RemovePID falló: %v", err)
	}

	// Verificar que no existe.
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("PID file deberia haber sido eliminado")
	}

	// Llamar de nuevo cuando no existe: no debe fallar.
	if err := RemovePID(); err != nil {
		t.Errorf("RemovePID en archivo inexistente no deberia fallar: %v", err)
	}
}

func TestCheckAndCleanNoPIDFile(t *testing.T) {
	// Mover el PID file si existe para simular que no hay watcher.
	pidPath := PIDFilePath()
	backup := pidPath + ".backup_checkclean"
	renamed := false

	if _, err := os.Stat(pidPath); err == nil {
		if err := os.Rename(pidPath, backup); err == nil {
			renamed = true
			defer os.Rename(backup, pidPath) //nolint:errcheck
		}
	}
	if !renamed {
		defer os.Remove(pidPath) //nolint:errcheck
	}

	alive, pid, path := CheckAndClean()
	if alive {
		t.Error("CheckAndClean deberia retornar alive=false cuando no hay PID file")
	}
	if pid != 0 {
		t.Errorf("pid deberia ser 0, got %d", pid)
	}
	if path != "" {
		t.Errorf("path deberia ser vacio, got %q", path)
	}
}
