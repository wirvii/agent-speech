# agent-speech

Plugin TTS para agentes de consola como Claude Code. Hace que tu terminal te hable de vuelta.

Lee las respuestas de Claude en voz alta usando motores TTS locales:
- **macOS**: `say` (preinstalado)
- **Linux**: `piper` (instalacion manual requerida)

---

## Instalacion

### Opcion 1: go install

```bash
go install github.com/wirvii/agent-speech@latest
```

### Opcion 2: Binario precompilado

Descarga el binario para tu plataforma desde [Releases](https://github.com/wirvii/agent-speech/releases).

```bash
# Linux amd64
curl -L https://github.com/wirvii/agent-speech/releases/latest/download/agent-speech_linux_amd64.tar.gz | tar xz
sudo mv agent-speech /usr/local/bin/

# macOS Apple Silicon
curl -L https://github.com/wirvii/agent-speech/releases/latest/download/agent-speech_darwin_arm64.tar.gz | tar xz
sudo mv agent-speech /usr/local/bin/
```

---

## Setup rapido

```bash
# Paso 1: Instalar agent-speech
go install github.com/wirvii/agent-speech@latest

# Paso 2 (solo Linux): Instalar piper
pip install piper-tts

# Paso 3: Configurar e integrar con Claude Code
agent-speech init
```

`agent-speech init` hace todo lo necesario:
- Detecta el motor TTS disponible
- Descarga el modelo de voz en espanol (solo Linux)
- Crea `~/.config/agent-speech/config.toml`
- Registra el hook Stop en `~/.claude/settings.json`

---

## Uso

### Modo pipe (manual)

```bash
# Leer markdown desde stdin
echo "# Hola **mundo**" | agent-speech

# Con flags
cat respuesta.md | agent-speech --lang en --voice Samantha --rate 200

# Leer archivo
agent-speech < mi-documento.md
```

### Integracion con Claude Code (automatico)

Una vez ejecutado `agent-speech init`, Claude Code llamara automaticamente a `agent-speech`
al terminar cada respuesta. No requiere intervencion manual.

```bash
# Activar
agent-speech on

# Desactivar
agent-speech off
```

### Listar voces disponibles

```bash
agent-speech voices
```

Salida en macOS:
```
Motor: say (macOS)

Idioma: es
  Paulina (default)
  Juan

Idioma: en
  Samantha (default)
  Alex
```

Salida en Linux:
```
Motor: piper (Linux)

Idioma: es
  es_MX-claude-high (default) [descargado]
  es_MX-ald-medium [no descargado]

Idioma: en
  en_US-lessac-medium (default) [no descargado]
```

### Descargar modelos piper (Linux)

```bash
# Descargar modelo especifico
agent-speech download es_MX-claude-high
agent-speech download en_US-lessac-medium
```

---

## Configuracion

Archivo: `~/.config/agent-speech/config.toml`

```toml
# Idioma por defecto: "es" o "en"
lang = "es"

# Voz especifica (vacio = default por idioma y motor)
# macOS: "Paulina", "Juan", "Samantha", "Alex"
# Linux: "es_MX-claude-high", "es_MX-ald-medium", "en_US-lessac-medium"
voice = ""

# Velocidad en palabras por minuto (0 = default del motor)
rate = 0

# Motor TTS: "auto" (detecta por OS), "say", "piper"
engine = "auto"

# Directorio de modelos piper (solo Linux)
piper_model_dir = "~/.local/share/agent-speech/models"

# Logs detallados a stderr
verbose = false
```

Los flags CLI tienen prioridad sobre el archivo de configuracion:

| Flag | Descripcion |
|------|-------------|
| `--lang <es\|en>` | Idioma |
| `--voice <name>` | Voz especifica |
| `--rate <int>` | Velocidad (palabras/minuto) |
| `--engine <auto\|say\|piper>` | Forzar motor |
| `--verbose` | Logs a stderr |

---

## Prerequisitos

### macOS

Ningun prerequisito adicional. `say` viene preinstalado en macOS.

### Linux

Requiere `piper` instalado y al menos uno de: `aplay`, `paplay`, o `ffplay`.

```bash
# Instalar piper (cualquier distro con pip)
pip install piper-tts

# Instalar aplay (ALSA utils)
# Fedora/RHEL:
sudo dnf install alsa-utils

# Ubuntu/Debian:
sudo apt install alsa-utils

# Arch:
sudo pacman -S alsa-utils
```

Si `piper` no esta instalado, `agent-speech` mostrara instrucciones claras.

---

## Privacidad

`agent-speech` no envia datos a ningun servicio externo. Todo el procesamiento es local:
- `say`: motor del sistema operativo, completamente local
- `piper`: motor de IA local, sin conexion a internet durante la reproduccion

La unica conexion de red es para descargar modelos desde Hugging Face (`agent-speech download`
o `agent-speech init` en Linux).

---

## Desarrollo

```bash
# Compilar
go build ./...

# Tests
go test ./...

# Lint
go vet ./...

# Compilar binario
go build -o agent-speech ./cmd/agent-speech
```

---

## Licencia

Ver [LICENSE](LICENSE).
