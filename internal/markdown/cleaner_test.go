package markdown_test

import (
	"testing"

	"github.com/wirvii/agent-speech/internal/markdown"
)

func TestClean_Headers(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"# Hola mundo", "Hola mundo"},
		{"## Subtitulo", "Subtitulo"},
		{"### Tres niveles", "Tres niveles"},
		{"# **Bold header**", "Bold header"},
	}
	for _, tc := range cases {
		got := markdown.Clean(tc.input)
		if got != tc.want {
			t.Errorf("Clean(%q): got %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestClean_Bold(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"**bold**", "bold"},
		{"texto **bold** aqui", "texto bold aqui"},
		{"__underscore bold__", "underscore bold"},
	}
	for _, tc := range cases {
		got := markdown.Clean(tc.input)
		if got != tc.want {
			t.Errorf("Clean(%q): got %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestClean_Italic(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"*italic*", "italic"},
		{"texto *italic* aqui", "texto italic aqui"},
		{"_underscore italic_", "underscore italic"},
	}
	for _, tc := range cases {
		got := markdown.Clean(tc.input)
		if got != tc.want {
			t.Errorf("Clean(%q): got %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestClean_InlineCode(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"`code`", "code"},
		{"usa `fmt.Println()` aqui", "usa fmt.Println() aqui"},
	}
	for _, tc := range cases {
		got := markdown.Clean(tc.input)
		if got != tc.want {
			t.Errorf("Clean(%q): got %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestClean_CodeBlock(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"```go\nfmt.Println()\n```", "bloque de codigo en go"},
		{"~~~python\nprint('hi')\n~~~", "bloque de codigo en python"},
		{"```\nsin lenguaje\n```", "bloque de codigo"},
		{"~~~\nsin lenguaje\n~~~", "bloque de codigo"},
	}
	for _, tc := range cases {
		got := markdown.Clean(tc.input)
		if got != tc.want {
			t.Errorf("Clean(%q): got %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestClean_Links(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"[texto](https://example.com)", "texto"},
		{"visita [aqui](https://example.com) para mas", "visita aqui para mas"},
	}
	for _, tc := range cases {
		got := markdown.Clean(tc.input)
		if got != tc.want {
			t.Errorf("Clean(%q): got %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestClean_Images(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"![alt text](img.png)", ""},
		{"texto ![logo](logo.png) mas texto", "texto mas texto"},
	}
	for _, tc := range cases {
		got := markdown.Clean(tc.input)
		if got != tc.want {
			t.Errorf("Clean(%q): got %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestClean_Lists(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"- item uno", "item uno"},
		{"* item dos", "item dos"},
		{"+ item tres", "item tres"},
		{"1. primer item", "primer item"},
		{"2. segundo item", "segundo item"},
	}
	for _, tc := range cases {
		got := markdown.Clean(tc.input)
		if got != tc.want {
			t.Errorf("Clean(%q): got %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestClean_Blockquote(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"> cita importante", "cita importante"},
		{"> **cita bold**", "cita bold"},
	}
	for _, tc := range cases {
		got := markdown.Clean(tc.input)
		if got != tc.want {
			t.Errorf("Clean(%q): got %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestClean_Separators(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"---", ""},
		{"***", ""},
		{"___", ""},
		{"----", ""},
	}
	for _, tc := range cases {
		got := markdown.Clean(tc.input)
		if got != tc.want {
			t.Errorf("Clean(%q): got %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestClean_Tables(t *testing.T) {
	input := "| Col1 | Col2 |\n|------|------|\n| A    | B    |"
	got := markdown.Clean(input)
	if got != "" {
		t.Errorf("Clean(table): got %q, want empty", got)
	}
}

func TestClean_MultipleEmptyLines(t *testing.T) {
	input := "linea uno\n\n\n\nlinea dos"
	got := markdown.Clean(input)
	// No debe haber mas de dos newlines consecutivos
	if got != "linea uno\n\nlinea dos" {
		t.Errorf("Clean(multiempty): got %q, want %q", got, "linea uno\n\nlinea dos")
	}
}

func TestClean_Combined(t *testing.T) {
	input := `# Titulo Principal

Este es un texto con **bold** y *italic*.

- Item uno con ` + "`codigo`" + `
- Item dos con [link](https://example.com)

` + "```go" + `
package main
` + "```"

	got := markdown.Clean(input)

	// Verificar que no hay markdown
	mustNotContain := []string{"#", "**", "*", "`", "[", "]", "(", ")"}
	for _, s := range mustNotContain {
		if contains(got, s) {
			t.Errorf("Clean resultado contiene markdown %q: %q", s, got)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && indexString(s, substr) >= 0
}

func indexString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
