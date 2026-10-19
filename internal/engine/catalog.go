// Package engine starts and stops the local inference servers profiles run on.
package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/Impre-visible/mihari/internal/devmode"
)

type Kind string

const (
	LlamaServer Kind = "llama-server"
	VLLMMLX     Kind = "vllm-mlx"
	Ollama      Kind = "ollama"
	LMStudio    Kind = "lmstudio"
	// A remote OpenAI-compatible API, OpenCode Go by default, for development
	// machines too small to run a model.
	OpenCode Kind = "opencode"
)

// Info describes an engine for the profiles screen.
type Info struct {
	Kind  Kind   `json:"kind"`
	Label string `json:"label"`
	// False when the engine does not run on this OS or architecture.
	Supported bool   `json:"supported"`
	Installed bool   `json:"installed"`
	Path      string `json:"path"`
	// How to install it, shown when Installed is false.
	Install string `json:"install"`
	// Placeholder for the profile's Model field.
	ModelHint string `json:"modelHint"`
	// Placeholder for the profile's extra arguments.
	ArgsHint string `json:"argsHint"`
	// The engine caps parallel jobs itself; otherwise Mihari does.
	NativeParallel bool `json:"nativeParallel"`
}

func catalog() []Info {
	appleSilicon := runtime.GOOS == "darwin" && runtime.GOARCH == "arm64"
	brew := runtime.GOOS == "darwin" || runtime.GOOS == "linux"

	llamaInstall := "Télécharge une release sur github.com/ggml-org/llama.cpp/releases."
	if brew {
		llamaInstall = "brew install llama.cpp"
	} else if runtime.GOOS == "windows" {
		llamaInstall = "winget install llama.cpp"
	}

	return []Info{
		{
			Kind: LlamaServer, Label: "llama.cpp (llama-server)", Supported: true,
			Install:        llamaInstall,
			ModelHint:      "org/depot-GGUF[:Q4_K_M] ou /chemin/modele.gguf",
			ArgsHint:       "-ngl 99 -fa on",
			NativeParallel: true,
		},
		{
			Kind: VLLMMLX, Label: "vllm-mlx", Supported: appleSilicon,
			Install:        "pip install vllm-mlx",
			ModelHint:      "mlx-community/Qwen3-30B-A3B-4bit",
			ArgsHint:       "--use-paged-cache",
			NativeParallel: true,
		},
		{
			Kind: Ollama, Label: "Ollama", Supported: true,
			Install:        "Télécharge Ollama sur ollama.com/download.",
			ModelHint:      "qwen3:32b",
			NativeParallel: true,
		},
		{
			Kind: LMStudio, Label: "LM Studio", Supported: true,
			Install:   "Installe LM Studio depuis lmstudio.ai, puis lance-le une fois pour installer la commande lms.",
			ModelHint: "qwen/qwen3-32b",
		},
		{
			Kind: OpenCode, Label: "OpenCode (dev, distant)", Supported: devmode.Enabled,
			Install:   "Renseigne une clé API OpenCode Go ou Zen (opencode.ai/auth) dans la page Dev.",
			ModelHint: "glm-5.3",
		},
	}
}

func binary(kind Kind) string {
	switch kind {
	case LMStudio:
		return "lms"
	default:
		return string(kind)
	}
}

// lookPath also checks where installers drop binaries outside the PATH a GUI
// app inherits, such as Homebrew's prefix on macOS.
func lookPath(name string) string {
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	home, _ := os.UserHomeDir()
	for _, dir := range []string{
		"/opt/homebrew/bin",
		"/usr/local/bin",
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".lmstudio", "bin"),
	} {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}

// Catalog lists every engine with whether it is installed here.
func Catalog() []Info {
	infos := catalog()
	for i := range infos {
		if infos[i].Kind == OpenCode {
			// Nothing to install, only a key to set.
			infos[i].Installed = devmode.Enabled && remoteKey() != ""
			continue
		}
		infos[i].Path = lookPath(binary(infos[i].Kind))
		infos[i].Installed = infos[i].Path != ""
	}
	return infos
}

func find(kind Kind) (Info, bool) {
	for _, info := range Catalog() {
		if info.Kind == kind {
			return info, true
		}
	}
	return Info{}, false
}
