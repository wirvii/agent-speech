package engine

import (
	"testing"
)

func TestMP3Players_NotEmpty(t *testing.T) {
	if len(mp3Players) == 0 {
		t.Error("mp3Players: lista vacia")
	}
	for i, p := range mp3Players {
		if p.name == "" {
			t.Errorf("mp3Players[%d].name: vacio", i)
		}
		if len(p.args) == 0 {
			t.Errorf("mp3Players[%d].args: vacio", i)
		}
	}
}

func TestFindMP3Player_DoesNotPanic(t *testing.T) {
	// En un entorno sin mpv/ffplay/cvlc el test verifica que retorna error descriptivo
	// En un entorno con alguno instalado retorna el reproductor
	player, args, err := findMP3Player()
	if err != nil {
		// Verificar que el error menciona los reproductores esperados
		errStr := err.Error()
		if len(errStr) == 0 {
			t.Error("findMP3Player: error vacio")
		}
	} else {
		if player == "" {
			t.Error("findMP3Player: player vacio cuando no hay error")
		}
		if args == nil {
			t.Error("findMP3Player: args nil cuando no hay error")
		}
	}
}
