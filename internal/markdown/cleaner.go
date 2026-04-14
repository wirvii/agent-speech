package markdown

import (
	"regexp"
	"strings"
)

var (
	// Bloques de codigo con lenguaje: ```lang o ~~~lang
	reCodeBlockLang = regexp.MustCompile("^(```|~~~)(\\w+)\\s*$")
	// Bloques de codigo sin lenguaje: ``` o ~~~
	reCodeBlockOpen = regexp.MustCompile("^(```|~~~)\\s*$")
	// Headers: # ## ### etc.
	reHeader = regexp.MustCompile(`^#{1,6}\s+(.+)$`)
	// Codigo inline
	reInlineCode = regexp.MustCompile("`([^`]+)`")
	// Links: [texto](url)
	reLink = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	// Imagenes: ![alt](url)
	reImage = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	// Listas: - item, * item, + item, 1. item
	reList = regexp.MustCompile(`^(\s*)([-*+]|\d+\.)\s+(.+)$`)
	// Blockquotes: > texto
	reQuote = regexp.MustCompile(`^>\s*(.*)$`)
	// Separadores horizontales: ---, ***, ___
	reSeparator = regexp.MustCompile(`^(---+|\*\*\*+|___+)\s*$`)
	// Filas de tabla: | col | col |
	reTable = regexp.MustCompile(`^\|.*\|`)
	// Lineas de separacion de tabla: |---|---|
	reTableSep = regexp.MustCompile(`^\|[\s\-:|]+\|`)
	// Negrita con remocion de marcadores dobles (simplificado para limpieza)
	reBoldStrip = regexp.MustCompile(`\*\*(.+?)\*\*`)
)

// Clean transforma markdown en texto apto para lectura en voz alta.
func Clean(input string) string {
	lines := strings.Split(input, "\n")
	var result []string
	inCodeBlock := false
	consecutiveEmpty := 0

	for _, line := range lines {
		// Detectar inicio/fin de bloques de codigo
		if reCodeBlockLang.MatchString(line) {
			if !inCodeBlock {
				m := reCodeBlockLang.FindStringSubmatch(line)
				if len(m) >= 3 && m[2] != "" {
					result = append(result, "bloque de codigo en "+m[2])
				} else {
					result = append(result, "bloque de codigo")
				}
				inCodeBlock = true
			} else {
				inCodeBlock = false
			}
			consecutiveEmpty = 0
			continue
		}

		if reCodeBlockOpen.MatchString(line) {
			if !inCodeBlock {
				result = append(result, "bloque de codigo")
				inCodeBlock = true
			} else {
				inCodeBlock = false
			}
			consecutiveEmpty = 0
			continue
		}

		// Dentro de un bloque de codigo: ignorar contenido
		if inCodeBlock {
			continue
		}

		// Separadores horizontales: omitir
		if reSeparator.MatchString(line) {
			continue
		}

		// Tablas: omitir completamente
		if reTableSep.MatchString(line) || reTable.MatchString(line) {
			continue
		}

		// Headers: quitar los #
		if reHeader.MatchString(line) {
			m := reHeader.FindStringSubmatch(line)
			if len(m) >= 2 {
				cleaned := cleanInline(m[1])
				if cleaned != "" {
					result = append(result, cleaned)
					consecutiveEmpty = 0
				}
			}
			continue
		}

		// Blockquotes: quitar el >
		if reQuote.MatchString(line) {
			m := reQuote.FindStringSubmatch(line)
			if len(m) >= 2 {
				cleaned := cleanInline(m[1])
				if cleaned != "" {
					result = append(result, cleaned)
					consecutiveEmpty = 0
				}
			}
			continue
		}

		// Listas: quitar bullet/numero
		if reList.MatchString(line) {
			m := reList.FindStringSubmatch(line)
			if len(m) >= 4 {
				cleaned := cleanInline(m[3])
				if cleaned != "" {
					result = append(result, cleaned)
					consecutiveEmpty = 0
				}
			}
			continue
		}

		// Linea vacia
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			consecutiveEmpty++
			if consecutiveEmpty == 1 {
				result = append(result, "")
			}
			continue
		}
		consecutiveEmpty = 0

		// Linea normal: limpiar inline
		cleaned := cleanInline(trimmed)
		if cleaned != "" {
			result = append(result, cleaned)
		}
	}

	// Unir y limpiar espacios extra al inicio/final
	text := strings.Join(result, "\n")
	text = strings.TrimSpace(text)

	// Colapsar multiples lineas vacias en una
	reMultiEmpty := regexp.MustCompile(`\n{3,}`)
	text = reMultiEmpty.ReplaceAllString(text, "\n\n")

	return text
}

// reMultiSpace colapsa espacios multiples en uno solo.
var reMultiSpace = regexp.MustCompile(`[ \t]{2,}`)

// cleanInline aplica transformaciones de formato inline.
func cleanInline(s string) string {
	// Imagenes primero (antes que links)
	s = reImage.ReplaceAllString(s, "")
	// Colapsar espacios multiples dejados por elementos eliminados
	s = reMultiSpace.ReplaceAllString(s, " ")

	// Links: [texto](url) → texto
	s = reLink.ReplaceAllStringFunc(s, func(m string) string {
		sub := reLink.FindStringSubmatch(m)
		if len(sub) >= 2 {
			return sub[1]
		}
		return ""
	})

	// Negrita: **texto** → texto
	s = reBoldStrip.ReplaceAllStringFunc(s, func(m string) string {
		sub := reBoldStrip.FindStringSubmatch(m)
		if len(sub) >= 2 {
			return sub[1]
		}
		return ""
	})
	// Negrita con __
	reUnderBold := regexp.MustCompile(`__(.+?)__`)
	s = reUnderBold.ReplaceAllStringFunc(s, func(m string) string {
		sub := reUnderBold.FindStringSubmatch(m)
		if len(sub) >= 2 {
			return sub[1]
		}
		return ""
	})

	// Cursiva: *texto* o _texto_ → texto
	reItalicAst := regexp.MustCompile(`\*([^*\n]+?)\*`)
	s = reItalicAst.ReplaceAllStringFunc(s, func(m string) string {
		sub := reItalicAst.FindStringSubmatch(m)
		if len(sub) >= 2 {
			return sub[1]
		}
		return ""
	})
	reItalicUnder := regexp.MustCompile(`_([^_\n]+?)_`)
	s = reItalicUnder.ReplaceAllStringFunc(s, func(m string) string {
		sub := reItalicUnder.FindStringSubmatch(m)
		if len(sub) >= 2 {
			return sub[1]
		}
		return ""
	})

	// Codigo inline: `texto` → texto
	s = reInlineCode.ReplaceAllStringFunc(s, func(m string) string {
		sub := reInlineCode.FindStringSubmatch(m)
		if len(sub) >= 2 {
			return sub[1]
		}
		return ""
	})

	return strings.TrimSpace(s)
}
