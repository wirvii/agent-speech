package engine

import (
	"fmt"
	"runtime"

	"github.com/wirvii/agent-speech/internal/config"
)

// Detect retorna el motor TTS adecuado segun la plataforma y la configuracion.
// Si cfg.Engine != "auto", fuerza ese motor.
// Verifica que el motor este disponible antes de retornarlo.
func Detect(cfg *config.Config) (Engine, error) {
	var eng Engine

	engineName := cfg.Engine
	if engineName == "" {
		engineName = "auto"
	}

	switch engineName {
	case "say":
		eng = &Say{}
	case "piper":
		modelDir, err := config.ExpandPath(cfg.PiperModelDir)
		if err != nil {
			modelDir = cfg.PiperModelDir
		}
		eng = &Piper{ModelDir: modelDir}
	case "auto":
		switch runtime.GOOS {
		case "darwin":
			eng = &Say{}
		case "linux":
			modelDir, err := config.ExpandPath(cfg.PiperModelDir)
			if err != nil {
				modelDir = cfg.PiperModelDir
			}
			eng = &Piper{ModelDir: modelDir}
		default:
			return nil, fmt.Errorf("plataforma no soportada: %s", runtime.GOOS)
		}
	default:
		return nil, fmt.Errorf("motor desconocido: %q (opciones: auto, say, piper)", engineName)
	}

	if !eng.Available() {
		return nil, unavailableError(eng)
	}

	return eng, nil
}

// unavailableError retorna un error descriptivo cuando un motor no esta disponible.
func unavailableError(eng Engine) error {
	switch eng.Name() {
	case "say":
		return fmt.Errorf(
			"motor 'say' no disponible.\n" +
				"  'say' es una herramienta de macOS. Verifica que estas en macOS.",
		)
	case "piper":
		return fmt.Errorf(
			"piper no encontrado en PATH.\n" +
				"  Instala piper para tu distribucion:\n\n" +
				"  Fedora/RHEL: pip install piper-tts\n" +
				"  Ubuntu/Debian: pip install piper-tts\n" +
				"  Arch: pip install piper-tts\n\n" +
				"  O descarga el binario de:\n" +
				"  https://github.com/OHF-Voice/piper1-gpl/releases",
		)
	default:
		return fmt.Errorf("motor %q no disponible", eng.Name())
	}
}
