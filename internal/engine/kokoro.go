package engine

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// Kokoro usa el CLI kokoro-tts (pip install kokoro-tts) para generar audio local.
// Requiere modelos ONNX descargados previamente (~500MB).
type Kokoro struct{}

// Name retorna el nombre del motor.
func (k *Kokoro) Name() string { return "kokoro" }

// Available retorna true si el CLI kokoro-tts esta en PATH.
func (k *Kokoro) Available() bool {
	_, err := exec.LookPath("kokoro-tts")
	return err == nil
}

// Speak genera audio con kokoro-tts y lo reproduce con el primer reproductor disponible.
// Estrategia: escribir texto a archivo temporal, generar WAV temporal, reproducir, limpiar.
func (k *Kokoro) Speak(ctx context.Context, text string, opts SpeakOpts) error {
	if text == "" {
		return nil
	}

	voice := opts.Voice
	if voice == "" {
		voice = DefaultVoiceKokoro(opts.Lang)
	}

	lang := kokoroLangCode(opts.Lang)

	// Crear archivo temporal para el texto (evitar problemas de escaping en args)
	txtFile, err := os.CreateTemp("", "agent-speech-*.txt")
	if err != nil {
		return fmt.Errorf("kokoro: crear archivo temporal de texto: %w", err)
	}
	defer os.Remove(txtFile.Name())

	if _, err := txtFile.WriteString(text); err != nil {
		txtFile.Close()
		return fmt.Errorf("kokoro: escribir texto: %w", err)
	}
	txtFile.Close()

	// Crear archivo temporal para el audio WAV
	wavFile, err := os.CreateTemp("", "agent-speech-*.wav")
	if err != nil {
		return fmt.Errorf("kokoro: crear archivo temporal de audio: %w", err)
	}
	wavPath := wavFile.Name()
	wavFile.Close()
	defer os.Remove(wavPath)

	// Paso 1: Generar audio con kokoro-tts
	genCmd := exec.CommandContext(ctx, "kokoro-tts",
		txtFile.Name(), wavPath,
		"--voice", voice,
		"--lang", lang,
	)
	if err := genCmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("kokoro: generar audio: %w", err)
	}

	// Paso 2: Reproducir el archivo WAV (mpv/ffplay manejan WAV tambien)
	player, playerArgs, err := findMP3Player()
	if err != nil {
		return fmt.Errorf("kokoro: %w", err)
	}

	args := append(playerArgs, wavPath) //nolint:gocritic
	playCmd := exec.CommandContext(ctx, player, args...)
	if err := playCmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("kokoro: reproducir: %w", err)
	}

	return nil
}

// DefaultVoiceKokoro retorna la voz default de kokoro segun idioma.
func DefaultVoiceKokoro(lang string) string {
	switch lang {
	case "en":
		return "af_heart"
	default: // "es" y cualquier otro
		return "ef_dora"
	}
}

// kokoroLangCode mapea nuestro codigo de idioma al formato que acepta kokoro-tts.
func kokoroLangCode(lang string) string {
	switch lang {
	case "en":
		return "en-us"
	case "es":
		return "es"
	default:
		return "es"
	}
}
