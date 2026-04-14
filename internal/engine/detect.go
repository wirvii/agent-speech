package engine

import (
	"fmt"
	"runtime"

	"github.com/wirvii/agent-speech/internal/config"
	"github.com/wirvii/agent-speech/internal/piper"
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
		p := &Piper{ModelDir: modelDir}
		if binPath, found := piper.BinPath(); found {
			p.BinPath = binPath
			binDir, _ := piper.BinDir()
			p.BinDir = binDir
		}
		eng = p
	case "auto":
		switch runtime.GOOS {
		case "darwin":
			eng = &Say{}
		case "linux":
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
			eng = p
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
			"piper no encontrado.\n" +
				"  Ejecuta 'agent-speech init' para instalarlo automaticamente.\n\n" +
				"  O instala piper manualmente:\n" +
				"  https://github.com/rhasspy/piper/releases",
		)
	default:
		return fmt.Errorf("motor %q no disponible", eng.Name())
	}
}
