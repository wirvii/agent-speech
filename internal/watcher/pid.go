package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// PIDFilePath retorna la ruta al PID file.
func PIDFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".local", "share", "agent-speech", "watcher.pid")
	}
	return filepath.Join(home, ".local", "share", "agent-speech", "watcher.pid")
}

// WritePID escribe el PID file con el PID actual y el transcript path.
// Usa escritura atomica: escribe a un temporal y luego os.Rename.
func WritePID(transcriptPath string) error {
	path := PIDFilePath()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("crear directorio para PID file: %w", err)
	}

	content := fmt.Sprintf("%d\n%s\n", os.Getpid(), transcriptPath)

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("escribir PID file temporal: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) //nolint:errcheck
		return fmt.Errorf("renombrar PID file: %w", err)
	}

	return nil
}

// ReadPID lee el PID file. Retorna pid, transcriptPath, error.
// Retorna error si el archivo no existe.
func ReadPID() (pid int, transcriptPath string, err error) {
	path := PIDFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, "", fmt.Errorf("leer PID file: %w", err)
	}

	lines := strings.SplitN(strings.TrimRight(string(data), "\n"), "\n", 2)
	if len(lines) < 2 {
		return 0, "", fmt.Errorf("formato de PID file invalido: se esperaban 2 lineas, got %d", len(lines))
	}

	pid, err = strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil {
		return 0, "", fmt.Errorf("parsear PID: %w", err)
	}

	transcriptPath = strings.TrimSpace(lines[1])
	return pid, transcriptPath, nil
}

// IsAlive verifica si el proceso con el PID dado esta vivo.
// Usa syscall.Kill(pid, 0) — funciona en Linux y macOS.
func IsAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil
}

// RemovePID elimina el PID file.
func RemovePID() error {
	path := PIDFilePath()
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("eliminar PID file: %w", err)
	}
	return nil
}

// CheckAndClean verifica si el watcher esta vivo.
// Si el PID file existe pero el proceso murio, lo limpia.
// Retorna (alive bool, pid int, transcriptPath string).
func CheckAndClean() (alive bool, pid int, transcriptPath string) {
	pid, transcriptPath, err := ReadPID()
	if err != nil {
		// No hay PID file o esta corrupto.
		return false, 0, ""
	}

	if IsAlive(pid) {
		return true, pid, transcriptPath
	}

	// El proceso murio, limpiar PID file huerfano.
	RemovePID() //nolint:errcheck
	return false, 0, ""
}

// KillExisting mata el watcher anterior si existe.
// Envía SIGTERM y espera hasta maxWait a que muera.
func KillExisting(pid int, maxWait time.Duration) error {
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		// Si ya no existe, no es un error.
		if err == syscall.ESRCH {
			return nil
		}
		return fmt.Errorf("enviar SIGTERM al watcher anterior (pid %d): %w", pid, err)
	}

	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		if !IsAlive(pid) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Si aun esta vivo despues de esperar, forzar con SIGKILL.
	syscall.Kill(pid, syscall.SIGKILL) //nolint:errcheck
	return nil
}
