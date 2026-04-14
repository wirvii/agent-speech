package engine

import (
	"fmt"
	"os/exec"
)

// mp3Player describe un reproductor capaz de reproducir archivos MP3/WAV.
type mp3Player struct {
	name string
	args []string // args ANTES del path del archivo
}

// mp3Players lista de reproductores en orden de preferencia.
var mp3Players = []mp3Player{
	{name: "mpv", args: []string{"--no-video", "--really-quiet"}},
	{name: "ffplay", args: []string{"-nodisp", "-autoexit", "-loglevel", "quiet"}},
	{name: "cvlc", args: []string{"--play-and-exit", "--quiet"}},
}

// findMP3Player busca el primer reproductor capaz de decodificar MP3/WAV.
// Compartido por EdgeTTS y Kokoro.
func findMP3Player() (string, []string, error) {
	for _, p := range mp3Players {
		if _, err := exec.LookPath(p.name); err == nil {
			return p.name, p.args, nil
		}
	}
	return "", nil, fmt.Errorf(
		"no se encontro reproductor de audio para MP3 (mpv, ffplay, cvlc).\n" +
			"  Instala mpv: sudo apt install mpv  o  sudo dnf install mpv",
	)
}
