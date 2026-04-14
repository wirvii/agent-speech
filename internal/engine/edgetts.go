package engine

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// EdgeTTS usa el CLI edge-tts (pip install edge-tts) para generar audio
// y mpv/ffplay/cvlc para reproducirlo.
type EdgeTTS struct{}

// Name retorna el nombre del motor.
func (e *EdgeTTS) Name() string { return "edge-tts" }

// Available retorna true si el CLI edge-tts esta en PATH.
func (e *EdgeTTS) Available() bool {
	_, err := exec.LookPath("edge-tts")
	return err == nil
}

// Speak genera audio con edge-tts y lo reproduce con el primer reproductor disponible.
// Estrategia: generar a archivo MP3 temporal, reproducir, limpiar.
func (e *EdgeTTS) Speak(ctx context.Context, text string, opts SpeakOpts) error {
	if text == "" {
		return nil
	}

	voice := opts.Voice
	if voice == "" {
		voice = DefaultVoiceEdgeTTS(opts.Lang)
	}

	// Crear archivo temporal para el audio generado
	tmpFile, err := os.CreateTemp("", "agent-speech-*.mp3")
	if err != nil {
		return fmt.Errorf("edge-tts: crear archivo temporal: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Paso 1: Generar audio con edge-tts
	args := []string{"--voice", voice, "--text", text, "--write-media", tmpPath}

	// Agregar rate: default +25% (mas rapido pero entendible); Rate>0 usa el valor directo como porcentaje.
	rate := "+25%"
	if opts.Rate > 0 {
		rate = fmt.Sprintf("+%d%%", opts.Rate)
	}
	args = append(args, "--rate", rate)

	genCmd := exec.CommandContext(ctx, "edge-tts", args...)
	if err := genCmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("edge-tts: generar audio: %w", err)
	}

	// Paso 2: Reproducir el archivo MP3
	player, playerArgs, err := findMP3Player()
	if err != nil {
		return fmt.Errorf("edge-tts: %w", err)
	}

	playArgs := make([]string, len(playerArgs)+1)
	copy(playArgs, playerArgs)
	playArgs[len(playerArgs)] = tmpPath
	playCmd := exec.CommandContext(ctx, player, playArgs...)
	if err := playCmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("edge-tts: reproducir: %w", err)
	}

	return nil
}

// DefaultVoiceEdgeTTS retorna la voz default de edge-tts segun idioma.
func DefaultVoiceEdgeTTS(lang string) string {
	switch lang {
	case "en":
		return "en-US-JennyNeural"
	default: // "es" y cualquier otro
		return "es-MX-DaliaNeural"
	}
}
