package watcher

import (
	"testing"
)

func TestSplitParagraphsEmpty(t *testing.T) {
	complete, residual := SplitParagraphs("")
	if len(complete) != 0 {
		t.Errorf("esperaba 0 parrafos completos, got %d", len(complete))
	}
	if residual != "" {
		t.Errorf("residual esperado vacio, got %q", residual)
	}
}

func TestSplitParagraphsSingleParagraph(t *testing.T) {
	text := "Hola mundo, esto es un parrafo simple."
	complete, residual := SplitParagraphs(text)

	// Un parrafo sin \n\n al final: es residual o completo dependiendo de si hay separador.
	// Como no hay \n\n, todo va como residual o como el unico parrafo completo.
	// El resultado esperado es que el texto aparezca como completo o residual (no vacio).
	total := len(complete) + func() int {
		if residual != "" {
			return 1
		}
		return 0
	}()
	if total != 1 {
		t.Errorf("esperaba 1 parte total, got %d (complete=%d, residual=%q)", total, len(complete), residual)
	}
}

func TestSplitParagraphsMultipleParagraphs(t *testing.T) {
	text := "Parrafo uno.\n\nParrafo dos.\n\nParrafo tres."
	complete, residual := SplitParagraphs(text)

	// "Parrafo tres." es residual (no hay \n\n al final).
	// "Parrafo uno." y "Parrafo dos." son completos.
	if len(complete) != 2 {
		t.Errorf("esperaba 2 parrafos completos, got %d: %v", len(complete), complete)
	}
	if complete[0] != "Parrafo uno." {
		t.Errorf("parrafo 0 esperado %q, got %q", "Parrafo uno.", complete[0])
	}
	if complete[1] != "Parrafo dos." {
		t.Errorf("parrafo 1 esperado %q, got %q", "Parrafo dos.", complete[1])
	}
	if residual != "Parrafo tres." {
		t.Errorf("residual esperado %q, got %q", "Parrafo tres.", residual)
	}
}

func TestSplitParagraphsEndsWithSeparator(t *testing.T) {
	text := "Parrafo uno.\n\nParrafo dos.\n\n"
	complete, residual := SplitParagraphs(text)

	if len(complete) != 2 {
		t.Errorf("esperaba 2 parrafos completos, got %d: %v", len(complete), complete)
	}
	if residual != "" {
		t.Errorf("residual deberia ser vacio cuando termina con \\n\\n, got %q", residual)
	}
}

func TestSplitParagraphsCodeBlockIndivisible(t *testing.T) {
	text := "Texto antes.\n\n```go\nfunc main() {\n\n\tfmt.Println(\"hola\")\n}\n```\n\nTexto despues."
	complete, residual := SplitParagraphs(text)

	// "Texto antes." es completo.
	// El bloque de codigo con texto dentro (incluyendo linea vacia) es completo.
	// "Texto despues." es residual (no hay \n\n al final).
	if len(complete) < 2 {
		t.Errorf("esperaba al menos 2 parrafos completos, got %d: %v", len(complete), complete)
	}
	if complete[0] != "Texto antes." {
		t.Errorf("parrafo 0 esperado %q, got %q", "Texto antes.", complete[0])
	}
	// El bloque de codigo debe estar en el segundo parrafo completo.
	found := false
	for _, p := range complete {
		if len(p) > 0 && (p[0] == '`' || containsCodeBlock(p)) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("bloque de codigo deberia aparecer como parrafo completo, parrafos: %v", complete)
	}
	if residual != "Texto despues." {
		t.Errorf("residual esperado %q, got %q", "Texto despues.", residual)
	}
}

func TestSplitParagraphsOnlyNewlines(t *testing.T) {
	text := "\n\n\n"
	complete, residual := SplitParagraphs(text)
	if len(complete) != 0 {
		t.Errorf("esperaba 0 parrafos completos con solo newlines, got %d: %v", len(complete), complete)
	}
	if residual != "" {
		t.Errorf("residual deberia estar vacio, got %q", residual)
	}
}

func TestSplitParagraphsTrimsWhitespace(t *testing.T) {
	text := "  Parrafo con espacios.  \n\n  Otro parrafo.  "
	complete, residual := SplitParagraphs(text)

	// Verificar que los parrafos estan trimmed.
	allParts := append(complete, residual)
	for _, p := range allParts {
		if p == "" {
			continue
		}
		if len(p) > 0 && (p[0] == ' ' || p[len(p)-1] == ' ') {
			t.Errorf("parrafo no esta trimmed: %q", p)
		}
	}
}

// containsCodeBlock verifica si un string contiene marcadores de bloque de codigo.
func containsCodeBlock(s string) bool {
	return len(s) >= 3 && (s[:3] == "```" || containsSubstring(s, "```"))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
