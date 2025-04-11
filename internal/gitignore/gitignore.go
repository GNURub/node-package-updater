package gitignore

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Patrones predeterminados que siempre se ignoran
var defaultPatterns = []string{
	"node_modules/",
	".git/",
	"dist/",
	"build/",
	".cache/",
	"coverage/",
	".DS_Store",
	".vscode/",
}

// Matcher representa un conjunto de patrones de gitignore
type Matcher struct {
	patterns []pattern
	basePath string
}

// pattern representa un patrón individual de gitignore
type pattern struct {
	pattern   string
	isNegated bool
}

// NewMatcher crea un nuevo Matcher basado en los archivos .gitignore encontrados en la ruta base
func NewMatcher(basePath string) (*Matcher, error) {
	matcher := &Matcher{
		patterns: []pattern{},
		basePath: basePath,
	}

	// Añadir patrones predeterminados
	for _, p := range defaultPatterns {
		matcher.patterns = append(matcher.patterns, pattern{
			pattern:   p,
			isNegated: false,
		})
	}

	// Comprobar si existe un archivo .gitignore
	gitignorePath := filepath.Join(basePath, ".gitignore")
	if _, err := os.Stat(gitignorePath); err == nil {
		if err := matcher.loadPatterns(gitignorePath); err != nil {
			return nil, err
		}
	}

	return matcher, nil
}

// loadPatterns carga los patrones desde un archivo .gitignore
func (m *Matcher) loadPatterns(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		isNegated := strings.HasPrefix(line, "!")
		if isNegated {
			line = line[1:]
		}

		m.patterns = append(m.patterns, pattern{
			pattern:   line,
			isNegated: isNegated,
		})
	}

	return scanner.Err()
}

// ShouldIgnore determina si una ruta debería ignorarse según los patrones de gitignore
func (m *Matcher) ShouldIgnore(path string) bool {
	// Normalizar la ruta para que sea relativa a la ruta base
	relPath, err := filepath.Rel(m.basePath, path)
	if err != nil {
		return false
	}

	// Asegurarse de que la ruta use barras diagonales hacia adelante
	relPath = filepath.ToSlash(relPath)

	ignored := false
	for _, p := range m.patterns {
		if match(relPath, p.pattern) {
			ignored = !p.isNegated

			if ignored {
				break
			}
		}
	}

	return ignored
}

// match comprueba si una ruta coincide con un patrón de gitignore
func match(path, pattern string) bool {
	// Implementación básica de coincidencia de patrones de gitignore
	// Manejar patrones con comodines y otras características de gitignore

	// Verificar si el patrón termina con "/"
	if strings.HasSuffix(pattern, string(os.PathSeparator)) {
		if !strings.HasSuffix(path, string(os.PathSeparator)) {
			path += string(os.PathSeparator)
		}
		// Solo coincide con directorios
		return strings.Contains(path, pattern)
	}

	// Manejar el caso de coincidencia exacta
	if path == pattern {
		return true
	}

	// Manejar patrones con comodín al inicio
	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(path, pattern[1:])
	}

	// Manejar patrones con comodín al final
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(path, pattern[:len(pattern)-1])
	}

	// Manejar patrones con comodín en medio
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "*")
		return strings.HasPrefix(path, parts[0]) && strings.HasSuffix(path, parts[1])
	}

	// Manejar patrones que representan directorios en cualquier nivel
	if strings.HasPrefix(pattern, "**") {
		pattern = pattern[2:]
		return strings.Contains(path, pattern)
	}

	// Por defecto, comprobar si la ruta comienza con el patrón
	return strings.HasPrefix(path, pattern)
}
