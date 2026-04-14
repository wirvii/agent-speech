package engine

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/wirvii/agent-speech/internal/piper"
)

// audioPlayer describe un reproductor de audio disponible.
type audioPlayer struct {
	name string
	args []string
}

// audioPlayers lista de reproductores en orden de preferencia.
var audioPlayers = []audioPlayer{
	{
		name: "aplay",
		args: []string{"-r", "22050", "-f", "S16_LE", "-t", "raw", "-q", "-"},
	},
	{
		name: "paplay",
		args: []string{"--raw", "--rate=22050", "--format=s16le", "--channels=1"},
	},
	{
		name: "ffplay",
		args: []string{"-nodisp", "-autoexit", "-f", "s16le", "-ar", "22050", "-ac", "1", "-"},
	},
}

// Piper es el motor TTS para Linux usando el binario piper.
type Piper struct {
	ModelDir string
}

// Name retorna el nombre del motor.
func (p *Piper) Name() string { return "piper" }

// Available retorna true si el binario piper esta en PATH.
// No verifica modelos, solo que el binario exista.
func (p *Piper) Available() bool {
	_, err := exec.LookPath("piper")
	return err == nil
}

// Speak reproduce el texto usando piper y aplay (o alternativa).
func (p *Piper) Speak(ctx context.Context, text string, opts SpeakOpts) error {
	if text == "" {
		return nil
	}

	modelDir := p.ModelDir
	if modelDir == "" {
		modelDir = "~/.local/share/agent-speech/models"
	}

	// Resolver modelo
	modelInfo, err := piper.Resolve(opts.Lang, opts.Voice)
	if err != nil {
		return fmt.Errorf("piper: resolver modelo: %w", err)
	}

	modelPath, err := piper.ModelPath(modelInfo.ID, modelDir)
	if err != nil {
		return fmt.Errorf("piper: modelo %q no encontrado en %s — ejecuta 'agent-speech download %s' para descargarlo",
			modelInfo.ID, modelDir, modelInfo.ID)
	}

	// Encontrar reproductor de audio disponible
	player, err := findAudioPlayer()
	if err != nil {
		return fmt.Errorf("piper: %w", err)
	}

	return p.runPipeline(ctx, text, modelPath, player)
}

// runPipeline ejecuta piper | aplay (o equivalente) via pipes de Go.
func (p *Piper) runPipeline(ctx context.Context, text, modelPath string, player audioPlayer) error {
	piperCmd := exec.CommandContext(ctx, "piper",
		"-m", modelPath,
		"--output-raw",
	)
	piperCmd.Stdin = strings.NewReader(text)

	piperOut, err := piperCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("piper: stdout pipe: %w", err)
	}

	playerCmd := exec.CommandContext(ctx, player.name, player.args...)
	playerCmd.Stdin = piperOut

	if err := playerCmd.Start(); err != nil {
		return fmt.Errorf("piper: iniciar %s: %w", player.name, err)
	}

	if err := piperCmd.Run(); err != nil {
		if ctx.Err() != nil {
			_ = playerCmd.Process.Kill()
			return nil
		}
		return fmt.Errorf("piper: ejecutar: %w", err)
	}

	if err := playerCmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("piper: %s: %w", player.name, err)
	}

	return nil
}

// findAudioPlayer busca el primer reproductor de audio disponible.
func findAudioPlayer() (audioPlayer, error) {
	for _, ap := range audioPlayers {
		if _, err := exec.LookPath(ap.name); err == nil {
			return ap, nil
		}
	}
	return audioPlayer{}, fmt.Errorf(
		"no se encontro reproductor de audio (aplay, paplay, ffplay).\n" +
			"  Instala ALSA utils: sudo apt install alsa-utils  o  sudo dnf install alsa-utils",
	)
}
