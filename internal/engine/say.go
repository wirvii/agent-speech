package engine

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

const (
	sayMaxChunkBytes = 200 * 1024 // 200KB
)

// Say es el motor TTS para macOS usando el comando say.
type Say struct{}

// Name retorna el nombre del motor.
func (s *Say) Name() string { return "say" }

// Available retorna true si el comando say esta en PATH.
func (s *Say) Available() bool {
	_, err := exec.LookPath("say")
	return err == nil
}

// Speak reproduce el texto usando say en macOS.
// Si el texto supera 200KB, lo parte en chunks por parrafos.
func (s *Say) Speak(ctx context.Context, text string, opts SpeakOpts) error {
	if text == "" {
		return nil
	}

	voice := opts.Voice
	if voice == "" {
		voice = DefaultVoiceSay(opts.Lang)
	}

	chunks := splitText(text, sayMaxChunkBytes)
	for _, chunk := range chunks {
		if err := ctx.Err(); err != nil {
			return nil // interrupcion por senial, no es error
		}
		if err := s.speakChunk(ctx, chunk, voice, opts.Rate); err != nil {
			return err
		}
	}
	return nil
}

// speakChunk ejecuta say para un fragmento de texto.
func (s *Say) speakChunk(ctx context.Context, text, voice string, rate int) error {
	args := []string{}
	if voice != "" {
		args = append(args, "-v", voice)
	}
	if rate > 0 {
		args = append(args, "-r", fmt.Sprintf("%d", rate))
	}
	args = append(args, text)

	cmd := exec.CommandContext(ctx, "say", args...)
	if err := cmd.Run(); err != nil {
		// Si el contexto fue cancelado, no es error
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("say: %w", err)
	}
	return nil
}

// splitText divide el texto en chunks de maximo maxBytes bytes,
// intentando cortar por parrafos cuando sea posible.
func splitText(text string, maxBytes int) []string {
	if len(text) <= maxBytes {
		return []string{text}
	}

	paragraphs := strings.Split(text, "\n\n")
	var chunks []string
	var current strings.Builder

	for _, para := range paragraphs {
		if current.Len()+len(para)+2 > maxBytes && current.Len() > 0 {
			chunks = append(chunks, strings.TrimSpace(current.String()))
			current.Reset()
		}
		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(para)
	}

	if current.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(current.String()))
	}

	return chunks
}
