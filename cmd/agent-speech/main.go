package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/wirvii/agent-speech/internal/config"
	"github.com/wirvii/agent-speech/internal/engine"
	"github.com/wirvii/agent-speech/internal/hook"
	"github.com/wirvii/agent-speech/internal/markdown"
	"github.com/wirvii/agent-speech/internal/piper"
	"github.com/wirvii/agent-speech/internal/updater"
)

// hookInput es el JSON que Claude Code pasa por stdin al disparar el hook Stop.
type hookInput struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	HookEventName  string `json:"hook_event_name"`
}

// variables inyectadas por ldflags en tiempo de compilacion (goreleaser).
var (
	version = "dev"
	commit  = ""
	date    = ""
)

// flags globales
var (
	flagLang     string
	flagVoice    string
	flagRate     int
	flagEngine   string
	flagVerbose  bool
	flagFromHook bool
)

func main() {
	rootCmd := buildRootCmd()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func buildRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "agent-speech",
		Short: "Plugin TTS para agentes de consola",
		Long: `agent-speech lee markdown desde stdin y lo reproduce en voz alta.

Se integra con Claude Code via hooks para leer automaticamente las respuestas del agente.

Ejemplos:
  echo "# Hola mundo" | agent-speech
  cat respuesta.md | agent-speech --lang en --voice Samantha
  agent-speech init`,
		RunE:    runSpeak,
		Version: version,
	}
	rootCmd.SetVersionTemplate(fmt.Sprintf("agent-speech %s (commit: %s, built: %s)\n", version, commit, date))

	// Flags globales
	rootCmd.PersistentFlags().StringVar(&flagLang, "lang", "", "idioma (es|en)")
	rootCmd.PersistentFlags().StringVar(&flagVoice, "voice", "", "voz especifica")
	rootCmd.PersistentFlags().IntVar(&flagRate, "rate", 0, "velocidad en palabras por minuto")
	rootCmd.PersistentFlags().StringVar(&flagEngine, "engine", "", "motor TTS (auto|say|piper)")
	rootCmd.PersistentFlags().BoolVar(&flagVerbose, "verbose", false, "logs a stderr")
	rootCmd.PersistentFlags().BoolVar(&flagFromHook, "from-hook", false, "modo hook de Claude Code")

	// Subcomandos
	rootCmd.AddCommand(
		buildInitCmd(),
		buildOnCmd(),
		buildOffCmd(),
		buildVoicesCmd(),
		buildDownloadCmd(),
		buildUpdateCmd(),
	)

	return rootCmd
}

// runSpeak es el comando principal: lee stdin y habla.
func runSpeak(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("configuracion: %w", err)
	}

	applyFlags(cfg)
	setupLogging(cfg)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	var text string

	if flagFromHook {
		text, err = readFromHook()
		if err != nil {
			fmt.Fprintf(os.Stderr, "agent-speech: error en modo hook: %v\n", err)
			os.Exit(1)
		}
	} else {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("leer stdin: %w", err)
		}
		text = string(data)
	}

	if text == "" {
		if !flagFromHook {
			cmd.Help() //nolint:errcheck
		}
		return nil
	}

	cleanText := markdown.Clean(text)
	if cleanText == "" {
		if cfg.Verbose {
			log.Println("texto vacio despues de limpiar markdown")
		}
		return nil
	}

	if cfg.Verbose {
		log.Printf("texto limpio (%d chars): %s...", len(cleanText), truncate(cleanText, 50))
	}

	eng, err := engine.Detect(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "agent-speech: %v\n", err)
		os.Exit(1)
	}

	if cfg.Verbose {
		log.Printf("motor: %s", eng.Name())
	}

	opts := engine.SpeakOpts{
		Lang:  cfg.Lang,
		Voice: cfg.Voice,
		Rate:  cfg.Rate,
	}

	if err := eng.Speak(ctx, cleanText, opts); err != nil {
		if ctx.Err() != nil {
			return nil // interrupcion por SIGINT/SIGTERM
		}
		return fmt.Errorf("hablar: %w", err)
	}

	return nil
}

// readFromHook lee el JSON de stdin del hook de Claude Code y extrae el ultimo mensaje.
func readFromHook() (string, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("leer stdin: %w", err)
	}

	var input hookInput
	if err := json.Unmarshal(data, &input); err != nil {
		return "", fmt.Errorf("parsear JSON del hook: %w", err)
	}

	if input.TranscriptPath == "" {
		return "", fmt.Errorf("transcript_path vacio en el JSON del hook")
	}

	return hook.ExtractLastAssistantMessage(input.TranscriptPath)
}

// buildInitCmd construye el subcomando init.
func buildInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Configura agent-speech e integra con Claude Code",
		Long: `Detecta la plataforma, configura el motor TTS, crea config.toml,
y registra el hook Stop en Claude Code settings.json.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit()
		},
	}
}

// runInit ejecuta la inicializacion completa.
func runInit() error {
	fmt.Println("Inicializando agent-speech...")
	fmt.Println()

	cfg, err := loadConfig()
	if err != nil {
		cfg = config.Defaults()
	}
	applyFlags(cfg)

	// Paso 1: En Linux, asegurar que piper esta disponible
	if runtime.GOOS == "linux" {
		if err := ensurePiper(); err != nil {
			fmt.Fprintf(os.Stderr, "x %v\n", err)
			return nil
		}
	}

	// Paso 2: Detectar motor (ahora debe encontrar piper)
	eng, err := engine.Detect(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "x %v\n", err)
		return nil
	}
	fmt.Printf("ok Motor detectado: %s\n", motorDisplayName(eng))

	// Paso 3: Mostrar voz por defecto
	defaultVoice := defaultVoiceForEngine(eng, cfg)
	fmt.Printf("ok Voz por defecto: %s (%s)\n", defaultVoice, langName(cfg.Lang))

	// Paso 4: En Linux, verificar y descargar modelo piper
	if eng.Name() == "piper" {
		if err := initPiperModel(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "x %v\n", err)
		}
	}

	// Paso 5: Crear config.toml
	cfgPath, _ := config.ConfigPath()
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := config.WriteDefaults(); err != nil {
			fmt.Fprintf(os.Stderr, "x Error creando config: %v\n", err)
		} else {
			fmt.Printf("ok Config creada: %s\n", cfgPath)
		}
	} else {
		fmt.Printf("ok Config existente: %s\n", cfgPath)
	}

	// Paso 6: Configurar hook en Claude Code
	if err := hook.Enable(); err != nil {
		fmt.Fprintf(os.Stderr, "x Error configurando hook: %v\n", err)
	} else {
		fmt.Println("ok Hook configurado en Claude Code")
	}

	// Paso 7: Instalar slash commands
	if err := hook.InstallCommands(); err != nil {
		fmt.Fprintf(os.Stderr, "x Error instalando commands: %v\n", err)
	} else {
		fmt.Println("ok Commands instalados: /speak-on, /speak-off, /speak-voices")
	}

	fmt.Println()
	fmt.Println("  agent-speech esta activo. Claude te hablara al terminar cada respuesta.")
	fmt.Println("  Usa 'agent-speech off' para desactivar.")
	return nil
}

// ensurePiper verifica que piper este disponible, instalandolo si es necesario.
func ensurePiper() error {
	// 1. Buscar en PATH
	if _, err := exec.LookPath("piper"); err == nil {
		fmt.Println("ok Motor detectado: piper (en PATH)")
		return nil
	}

	// 2. Buscar en directorio interno
	if _, found := piper.BinPath(); found {
		fmt.Println("ok Motor detectado: piper (instalado internamente)")
		return nil
	}

	// 3. Descargar e instalar
	fmt.Println("  piper no encontrado en PATH, descargando...")
	fmt.Printf("  Descargando piper para linux/%s...\n", runtime.GOARCH)

	binPath, err := piper.Install()
	if err != nil {
		return fmt.Errorf("instalar piper: %w\n  Descarga manual: https://github.com/rhasspy/piper/releases", err)
	}

	fmt.Printf("ok piper instalado en %s\n", filepath.Dir(binPath))
	return nil
}

// initPiperModel verifica y descarga el modelo piper por defecto.
func initPiperModel(cfg *config.Config) error {
	modelInfo, err := piper.Resolve(cfg.Lang, "")
	if err != nil {
		return err
	}

	modelDir, _ := config.ExpandPath(cfg.PiperModelDir)

	_, err = piper.ModelPath(modelInfo.ID, modelDir)
	if err == nil {
		fmt.Printf("✓ Modelo piper: %s (ya descargado)\n", modelInfo.ID)
		return nil
	}

	fmt.Printf("  Descargando modelo piper %s...\n", modelInfo.ID)
	if err := piper.Download(modelInfo.ID, modelDir); err != nil {
		return fmt.Errorf("descargar modelo piper: %w", err)
	}
	fmt.Println()
	fmt.Printf("✓ Modelo piper descargado: %s\n", modelInfo.ID)
	return nil
}

// buildOnCmd construye el subcomando on.
func buildOnCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "on",
		Short: "Activa el hook de agent-speech en Claude Code",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := hook.Enable(); err != nil {
				return err
			}
			fmt.Println("✓ agent-speech activado")
			return nil
		},
	}
}

// buildOffCmd construye el subcomando off.
func buildOffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "off",
		Short: "Desactiva el hook de agent-speech en Claude Code",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := hook.Disable(); err != nil {
				return err
			}
			fmt.Println("✓ agent-speech desactivado")
			return nil
		},
	}
}

// buildVoicesCmd construye el subcomando voices.
func buildVoicesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "voices",
		Short: "Lista las voces disponibles para el motor actual",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVoices()
		},
	}
}

// runVoices lista las voces disponibles.
func runVoices() error {
	cfg, err := loadConfig()
	if err != nil {
		cfg = config.Defaults()
	}
	applyFlags(cfg)

	eng, err := engine.Detect(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "agent-speech: %v\n", err)
		os.Exit(1)
	}

	switch eng.Name() {
	case "say":
		printSayVoices()
	case "piper":
		printPiperVoices(cfg)
	}
	return nil
}

// printSayVoices lista las voces de say en macOS.
func printSayVoices() {
	fmt.Println("Motor: say (macOS)")
	fmt.Println()
	fmt.Println("Idioma: es")
	fmt.Println("  Paulina (default)")
	fmt.Println("  Juan")
	fmt.Println()
	fmt.Println("Idioma: en")
	fmt.Println("  Samantha (default)")
	fmt.Println("  Alex")
}

// printPiperVoices lista los modelos piper disponibles.
func printPiperVoices(cfg *config.Config) {
	fmt.Println("Motor: piper (Linux)")

	modelDir, _ := config.ExpandPath(cfg.PiperModelDir)

	fmt.Println()
	fmt.Println("Idioma: es")
	printPiperModel("es_MX-claude-high", modelDir, true)
	printPiperModel("es_MX-ald-medium", modelDir, false)

	fmt.Println()
	fmt.Println("Idioma: en")
	printPiperModel("en_US-lessac-medium", modelDir, true)
}

// printPiperModel imprime un modelo con estado de descarga.
func printPiperModel(id, modelDir string, isDefault bool) {
	_, err := piper.ModelPath(id, modelDir)
	status := "[descargado]"
	if err != nil {
		status = "[no descargado]"
	}
	defaultMark := ""
	if isDefault {
		defaultMark = " (default)"
	}
	fmt.Printf("  %s%s %s\n", id, defaultMark, status)
}

// buildDownloadCmd construye el subcomando download.
func buildDownloadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "download <model-id>",
		Short: "Descarga un modelo piper",
		Long: `Descarga un modelo piper al directorio de modelos.
Solo relevante en Linux.

Modelos disponibles:
  es_MX-claude-high  (español mexicano, alta calidad)
  es_MX-ald-medium   (español mexicano, calidad media)
  en_US-lessac-medium (ingles americano, calidad media)`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDownload(args[0])
		},
	}
}

// runDownload descarga un modelo piper especifico.
func runDownload(modelID string) error {
	cfg, err := loadConfig()
	if err != nil {
		cfg = config.Defaults()
	}
	applyFlags(cfg)

	modelDir, err := config.ExpandPath(cfg.PiperModelDir)
	if err != nil {
		modelDir = cfg.PiperModelDir
	}

	if err := piper.Download(modelID, modelDir); err != nil {
		fmt.Fprintf(os.Stderr, "✗ %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("✓ Modelo descargado en %s\n", modelDir)
	return nil
}

// loadConfig carga la configuracion desde archivo.
func loadConfig() (*config.Config, error) {
	return config.Load()
}

// applyFlags aplica los flags CLI sobre la configuracion.
func applyFlags(cfg *config.Config) {
	if flagLang != "" {
		cfg.Lang = flagLang
	}
	if flagVoice != "" {
		cfg.Voice = flagVoice
	}
	if flagRate != 0 {
		cfg.Rate = flagRate
	}
	if flagEngine != "" {
		cfg.Engine = flagEngine
	}
	if flagVerbose {
		cfg.Verbose = true
	}
}

// setupLogging configura el logger segun verbose.
func setupLogging(cfg *config.Config) {
	if cfg.Verbose {
		log.SetOutput(os.Stderr)
		log.SetPrefix("[agent-speech] ")
		log.SetFlags(0)
	} else {
		log.SetOutput(io.Discard)
	}
}

// motorDisplayName retorna nombre legible del motor.
func motorDisplayName(eng engine.Engine) string {
	switch eng.Name() {
	case "say":
		return "say (macOS)"
	case "piper":
		return "piper (Linux)"
	default:
		return eng.Name()
	}
}

// defaultVoiceForEngine retorna la voz por defecto para el motor y idioma.
func defaultVoiceForEngine(eng engine.Engine, cfg *config.Config) string {
	if eng.Name() == "say" {
		return engine.DefaultVoiceSay(cfg.Lang)
	}
	// piper
	info, err := piper.Resolve(cfg.Lang, "")
	if err != nil {
		return "desconocida"
	}
	return info.ID
}

// langName retorna el nombre del idioma.
func langName(lang string) string {
	switch lang {
	case "es":
		return "español"
	case "en":
		return "inglés"
	default:
		return lang
	}
}

// buildUpdateCmd construye el subcomando update.
func buildUpdateCmd() *cobra.Command {
	var flagForce bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Actualiza agent-speech a la ultima version disponible",
		Long: `Consulta GitHub releases, descarga el binario mas reciente para esta
plataforma y lo reemplaza atomicamente. Luego ejecuta init como post-install.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(flagForce)
		},
	}
	cmd.Flags().BoolVar(&flagForce, "force", false, "actualizar aunque ya tengas la ultima version")
	return cmd
}

const agentSpeechRepo = "wirvii/agent-speech"

// runUpdate ejecuta el flujo de actualizacion.
func runUpdate(force bool) error {
	fmt.Printf("  Version actual: %s\n", version)

	latest, err := updater.CheckLatest(agentSpeechRepo)
	if err != nil {
		return fmt.Errorf("consultar ultima version: %w", err)
	}
	fmt.Printf("  Ultima version: %s\n", latest)

	if !updater.NeedsUpdate(version, latest) && !force {
		fmt.Printf("ok Ya tienes la ultima version\n")
		return nil
	}

	fmt.Printf("  Descargando agent-speech %s para %s/%s...\n", latest, runtime.GOOS, runtime.GOARCH)

	if err := updater.Update(agentSpeechRepo, latest); err != nil {
		return fmt.Errorf("actualizar binario: %w", err)
	}
	fmt.Println("ok Binario actualizado")

	fmt.Println("  Ejecutando post-install...")
	if err := runInit(); err != nil {
		fmt.Fprintf(os.Stderr, "x Error en post-install: %v\n", err)
	}

	fmt.Printf("ok agent-speech actualizado a %s\n", latest)
	return nil
}

// truncate trunca un string a maxLen caracteres.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
