package engine

import (
	"context"
	"fmt"
	"os"
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
	BinPath  string // ruta al binario piper (vacio = buscar en PATH)
	BinDir   string // directorio del binario (para LD_LIBRARY_PATH)
}

// Name retorna el nombre del motor.
func (p *Piper) Name() string { return "piper" }

// Available retorna true si el binario piper esta disponible.
// Orden de busqueda: BinPath explicito -> PATH del sistema -> directorio interno.
func (p *Piper) Available() bool {
	// 1. Si BinPath esta seteado, verificar que el archivo exista
	if p.BinPath != "" {
		_, err := os.Stat(p.BinPath)
		return err == nil
	}
	// 2. Buscar en PATH del sistema
	if _, err := exec.LookPath("piper"); err == nil {
		return true
	}
	// 3. Buscar en directorio interno
	binPath, found := piper.BinPath()
	if found {
		p.BinPath = binPath
		binDir, _ := piper.BinDir()
		p.BinDir = binDir
		return true
	}
	return false
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

// appendLDLibraryPath retorna os.Environ() con LD_LIBRARY_PATH prepended del directorio dado.
func appendLDLibraryPath(dir string) []string {
	env := os.Environ()
	existing := os.Getenv("LD_LIBRARY_PATH")
	var ldPath string
	if existing != "" {
		ldPath = dir + ":" + existing
	} else {
		ldPath = dir
	}
	found := false
	for i, e := range env {
		if strings.HasPrefix(e, "LD_LIBRARY_PATH=") {
			env[i] = "LD_LIBRARY_PATH=" + ldPath
			found = true
			break
		}
	}
	if !found {
		env = append(env, "LD_LIBRARY_PATH="+ldPath)
	}
	return env
}

// runPipeline ejecuta piper | aplay (o equivalente) via pipes de Go.
func (p *Piper) runPipeline(ctx context.Context, text, modelPath string, player audioPlayer) error {
	piperBin := "piper"
	if p.BinPath != "" {
		piperBin = p.BinPath
	}

	piperCmd := exec.CommandContext(ctx, piperBin,
		"-m", modelPath,
		"--output-raw",
	)
	piperCmd.Stdin = strings.NewReader(text)

	// Si usamos piper interno, setear LD_LIBRARY_PATH
	if p.BinDir != "" {
		piperCmd.Env = appendLDLibraryPath(p.BinDir)
	}

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
