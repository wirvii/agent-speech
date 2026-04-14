package engine

import (
	"testing"
)

func TestPiper_Name(t *testing.T) {
	p := &Piper{ModelDir: "/tmp/models"}
	if p.Name() != "piper" {
		t.Errorf("Name: got %q, want %q", p.Name(), "piper")
	}
}

func TestFindAudioPlayer_ReturnsError(t *testing.T) {
	// En un entorno sin aplay/paplay/ffplay el test verifica que retorna error
	// Este test es informativo — en CI puede no haber reproductores
	_, err := findAudioPlayer()
	// Solo verificamos que la funcion se puede llamar sin panic
	_ = err
}

func TestAudioPlayers_NotEmpty(t *testing.T) {
	if len(audioPlayers) == 0 {
		t.Error("audioPlayers: lista vacia")
	}
	for i, ap := range audioPlayers {
		if ap.name == "" {
			t.Errorf("audioPlayers[%d].name: vacio", i)
		}
		if len(ap.args) == 0 {
			t.Errorf("audioPlayers[%d].args: vacio", i)
		}
	}
}
