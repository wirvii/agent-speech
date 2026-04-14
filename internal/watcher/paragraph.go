package watcher

import "strings"

// SplitParagraphs divide texto en parrafos.
// Un parrafo termina con doble newline (\n\n).
// Bloques de codigo (```) son una unidad indivisible (no se parten).
// Retorna los parrafos completos y el texto residual (parrafo incompleto).
func SplitParagraphs(text string) (complete []string, residual string) {
	if text == "" {
		return nil, ""
	}

	// Si el texto es solo whitespace, no hay nada que hablar.
	if strings.TrimSpace(text) == "" {
		return nil, ""
	}

	// Dividir por doble newline preservando bloques de codigo.
	// Estrategia: iterar con un estado de "dentro de bloque de codigo".
	parts := splitRespectingCodeBlocks(text)

	// Si splitRespectingCodeBlocks no encontro partes con contenido,
	// el texto completo es un unico parrafo residual.
	if len(parts) == 0 {
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			return nil, ""
		}
		return nil, trimmed
	}

	// Verificar si el texto termina con \n\n (parrafo cerrado) o no.
	// Si termina con \n\n, todos los partes son completos.
	// Si no, el ultimo fragmento es residual.
	endsWithSeparator := strings.HasSuffix(text, "\n\n") ||
		strings.HasSuffix(strings.TrimRight(text, " \t"), "\n\n")

	if endsWithSeparator {
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				complete = append(complete, p)
			}
		}
		return complete, ""
	}

	// El ultimo fragmento es residual.
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if i == len(parts)-1 {
			residual = p
		} else {
			complete = append(complete, p)
		}
	}

	return complete, residual
}

// splitRespectingCodeBlocks divide texto por \n\n respetando bloques de codigo.
// Los bloques de codigo (entre ```) nunca se parten aunque contengan \n\n.
func splitRespectingCodeBlocks(text string) []string {
	var parts []string
	inCodeBlock := false
	current := &strings.Builder{}

	lines := strings.Split(text, "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detectar apertura/cierre de bloque de codigo.
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inCodeBlock = !inCodeBlock
			current.WriteString(line)
			if i < len(lines)-1 {
				current.WriteByte('\n')
			}
			continue
		}

		if inCodeBlock {
			// Dentro de bloque: acumular sin partir.
			current.WriteString(line)
			if i < len(lines)-1 {
				current.WriteByte('\n')
			}
			continue
		}

		// Fuera de bloque: verificar si esta linea y la anterior son vacias (separator \n\n).
		current.WriteString(line)
		if i < len(lines)-1 {
			current.WriteByte('\n')
		}

		// Revisar si acumulamos un separador: linea vacia seguida de contenido futuro.
		// Partimos cuando encontramos una linea vacia que sigue a contenido no vacio.
		if trimmed == "" && current.Len() > 0 {
			// Mirar adelante: si la siguiente linea es tambien vacia o hay contenido, partir.
			if i+1 < len(lines) {
				// Hay mas contenido: guardar parte actual y empezar nueva.
				part := strings.TrimRight(current.String(), "\n")
				if strings.TrimSpace(part) != "" {
					parts = append(parts, part)
					current.Reset()
				}
			}
		}
	}

	// Agregar lo que quede.
	if remaining := strings.TrimRight(current.String(), "\n"); strings.TrimSpace(remaining) != "" {
		parts = append(parts, remaining)
	}

	return parts
}
