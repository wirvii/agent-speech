package engine

import "context"

// Engine abstrae un motor TTS local.
type Engine interface {
	// Speak reproduce el texto como audio.
	// Bloquea hasta que termina o el contexto se cancela.
	Speak(ctx context.Context, text string, opts SpeakOpts) error

	// Available retorna true si el motor esta instalado y funcional.
	Available() bool

	// Name retorna el nombre del motor ("say", "piper").
	Name() string
}

// SpeakOpts configura la reproduccion.
type SpeakOpts struct {
	Lang  string // "es" o "en"
	Voice string // nombre de voz especifica (opcional, usa default por idioma)
	Rate  int    // palabras por minuto (0 = default del motor)
}

// DefaultVoiceSay retorna la voz default para say en macOS segun idioma.
func DefaultVoiceSay(lang string) string {
	switch lang {
	case "en":
		return "Samantha"
	default:
		return "Paulina"
	}
}
