package engine

import (
	"fmt"
	"runtime"

	"github.com/wirvii/agent-speech/internal/config"
	"github.com/wirvii/agent-speech/internal/piper"
)

// Detect retorna el motor TTS adecuado segun la plataforma y la configuracion.
// Si cfg.Engine != "auto", fuerza ese motor y verifica disponibilidad.
func Detect(cfg *config.Config) (Engine, error) {
	engineName := cfg.Engine
	if engineName == "" {
		engineName = "auto"
	}

	switch engineName {
	case "say":
		eng := &Say{}
		if !eng.Available() {
			return nil, unavailableError(eng)
		}
		return eng, nil

	case "edge-tts":
		eng := &EdgeTTS{}
		if !eng.Available() {
			return nil, unavailableError(eng)
		}
		return eng, nil

	case "kokoro":
		eng := &Kokoro{}
		if !eng.Available() {
			return nil, unavailableError(eng)
		}
		return eng, nil

	case "piper":
		eng := buildPiper(cfg)
		if !eng.Available() {
			return nil, unavailableError(eng)
		}
		return eng, nil

	case "auto":
		return detectAuto(cfg)

	default:
		return nil, fmt.Errorf(
			"motor desconocido: %q (opciones: auto, say, edge-tts, kokoro, piper)",
			engineName,
		)
	}
}

// detectAuto prueba motores en orden de prioridad segun la plataforma.
func detectAuto(cfg *config.Config) (Engine, error) {
	switch runtime.GOOS {
	case "darwin":
		eng := &Say{}
		if eng.Available() {
			return eng, nil
		}
		// Fallback en macOS: edge-tts si esta instalado
		edgeEng := &EdgeTTS{}
		if edgeEng.Available() {
			return edgeEng, nil
		}
		return nil, unavailableError(eng)

	case "linux":
		// Prioridad: edge-tts > kokoro > piper
		candidates := []Engine{
			&EdgeTTS{},
			&Kokoro{},
			buildPiper(cfg),
		}
		for _, eng := range candidates {
			if eng.Available() {
				return eng, nil
			}
		}
		return nil, fmt.Errorf(
			"ningun motor TTS disponible.\n" +
				"  Instala edge-tts:  pip install edge-tts\n" +
				"  O instala kokoro:  pip install kokoro-tts\n" +
				"  O ejecuta:        agent-speech init",
		)

	default:
		return nil, fmt.Errorf("plataforma no soportada: %s", runtime.GOOS)
	}
}

// buildPiper construye una instancia de Piper con la config apropiada.
func buildPiper(cfg *config.Config) *Piper {
	modelDir, err := config.ExpandPath(cfg.PiperModelDir)
	if err != nil {
		modelDir = cfg.PiperModelDir
	}
	p := &Piper{ModelDir: modelDir}
	if binPath, found := piper.BinPath(); found {
		p.BinPath = binPath
		binDir, _ := piper.BinDir()
		p.BinDir = binDir
	}
	return p
}

// unavailableError retorna un error descriptivo cuando un motor no esta disponible.
func unavailableError(eng Engine) error {
	switch eng.Name() {
	case "say":
		return fmt.Errorf(
			"motor 'say' no disponible.\n" +
				"  'say' es una herramienta de macOS. Verifica que estas en macOS.",
		)
	case "edge-tts":
		return fmt.Errorf(
			"edge-tts no encontrado.\n" +
				"  Instala con: pip install edge-tts\n" +
				"  O con pipx: pipx install edge-tts",
		)
	case "kokoro":
		return fmt.Errorf(
			"kokoro-tts no encontrado.\n" +
				"  Instala con: pip install kokoro-tts\n" +
				"  Requiere Python 3.11-3.12 y modelos ONNX (~500MB)",
		)
	case "piper":
		return fmt.Errorf(
			"piper no encontrado.\n" +
				"  Ejecuta 'agent-speech init' para instalarlo automaticamente.\n\n" +
				"  O instala piper manualmente:\n" +
				"  https://github.com/rhasspy/piper/releases",
		)
	default:
		return fmt.Errorf("motor %q no disponible", eng.Name())
	}
}
