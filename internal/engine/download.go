package engine

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Presence says whether a profile's model can start without downloading.
type Presence string

const (
	PresenceReady Presence = "ready"
	// The model must be downloaded first.
	PresenceMissing Presence = "missing"
	// A local path that does not exist; there is nothing to download.
	PresenceNoFile Presence = "nofile"
	// The engine cannot tell, as LM Studio without its CLI.
	PresenceUnknown Presence = "unknown"
)

func isLocalPath(model string) bool {
	return strings.HasSuffix(strings.ToLower(model), ".gguf") || filepath.IsAbs(model) ||
		strings.HasPrefix(model, "./") || strings.HasPrefix(model, "~")
}

func presence(kind Kind, model string) Presence {
	model = strings.TrimSpace(model)
	if model == "" {
		return PresenceMissing
	}
	if kind == OpenCode {
		// Served remotely; startRemote checks it exists.
		return PresenceReady
	}
	if isLocalPath(model) {
		if _, err := os.Stat(model); err != nil {
			return PresenceNoFile
		}
		return PresenceReady
	}
	var err error
	switch kind {
	case LlamaServer:
		_, err = localGGUF(model)
	case VLLMMLX:
		_, err = localSnapshot(model)
	case Ollama:
		if _, statErr := os.Stat(ollamaManifest(model)); statErr != nil {
			err = errNotDownloaded
		}
	case LMStudio:
		return lmStudioPresence(model)
	}
	if err != nil {
		return PresenceMissing
	}
	return PresenceReady
}

// download fetches model for kind, reporting bytes as it goes; total is 0
// while unknown.
func download(ctx context.Context, kind Kind, model string, progress func(done, total int64)) error {
	model = strings.TrimSpace(model)
	if kind == OpenCode {
		return errors.New("un modèle distant ne se télécharge pas")
	}
	if isLocalPath(model) {
		return errors.New("un chemin local ne se télécharge pas")
	}
	switch kind {
	case LlamaServer:
		return downloadHF(ctx, model, true, progress)
	case VLLMMLX:
		return downloadHF(ctx, model, false, progress)
	case Ollama:
		return ollamaPull(ctx, model, progress)
	case LMStudio:
		return lmStudioGet(ctx, model)
	}
	return fmt.Errorf("moteur inconnu : %q", kind)
}

// ollamaManifest is where Ollama records a pulled model: name defaults to the
// "library" namespace and tag to "latest".
func ollamaManifest(model string) string {
	root := os.Getenv("OLLAMA_MODELS")
	if root == "" {
		home, _ := os.UserHomeDir()
		root = filepath.Join(home, ".ollama", "models")
	}
	name, tag, ok := strings.Cut(model, ":")
	if !ok {
		tag = "latest"
	}
	parts := strings.Split(name, "/")
	host := "registry.ollama.ai"
	if len(parts) > 1 && strings.Contains(parts[0], ".") {
		host, parts = parts[0], parts[1:]
	}
	if len(parts) == 1 {
		parts = append([]string{"library"}, parts...)
	}
	return filepath.Join(append(append([]string{root, "manifests", host}, parts...), tag)...)
}

// ollamaPull runs a throwaway `ollama serve` of Mihari's own to pull model,
// since the user's instance may not be running.
func ollamaPull(ctx context.Context, model string, progress func(done, total int64)) error {
	info, _ := find(Ollama)
	if !info.Installed {
		return fmt.Errorf("Ollama n'est pas installé. %s", info.Install)
	}
	port, err := freePort()
	if err != nil {
		return err
	}
	s := &Server{URL: fmt.Sprintf("http://127.0.0.1:%d", port), logs: &tail{}}
	s.cmd = exec.Command(info.Path, "serve")
	s.cmd.Env = append(os.Environ(), "OLLAMA_HOST=127.0.0.1:"+strconv.Itoa(port))
	s.cmd.Stdout, s.cmd.Stderr = s.logs, s.logs
	setProcessGroup(s.cmd)
	if err := s.cmd.Start(); err != nil {
		return err
	}
	pid := s.cmd.Process.Pid
	track(pid, info.Path)
	s.done = make(chan struct{})
	go func() { _ = s.cmd.Wait(); killGroup(pid); untrack(pid); close(s.done) }()
	defer s.Stop()

	readyCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := s.waitReady(readyCtx); err != nil {
		return err
	}

	body, _ := json.Marshal(map[string]any{"model": model, "stream": true})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.URL+"/api/pull", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// Each layer reports its own total and completed bytes.
	type layer struct{ done, total int64 }
	layers := map[string]layer{}
	scanner := bufio.NewScanner(res.Body)
	for scanner.Scan() {
		var line struct {
			Status    string `json:"status"`
			Digest    string `json:"digest"`
			Total     int64  `json:"total"`
			Completed int64  `json:"completed"`
			Error     string `json:"error"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}
		if line.Error != "" {
			if strings.Contains(line.Error, "file does not exist") {
				return ollamaNotFound(ctx, model)
			}
			return errors.New(line.Error)
		}
		if line.Digest != "" && line.Total > 0 {
			layers[line.Digest] = layer{line.Completed, line.Total}
			var done, total int64
			for _, l := range layers {
				done += l.done
				total += l.total
			}
			progress(done, total)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if _, err := os.Stat(ollamaManifest(model)); err != nil {
		return errors.New("Ollama a terminé sans enregistrer le modèle")
	}
	return nil
}

// ollamaNotFound explains a name Ollama's registry does not have, which is
// usually a Hugging Face name typed as is, and suggests names that do exist.
func ollamaNotFound(ctx context.Context, model string) error {
	var suggestions []string
	if _, name, ok := strings.Cut(model, "/"); ok && !strings.Contains(strings.Split(model, "/")[0], ".") {
		// "Qwen/Qwen3.5-0.8B" is "qwen3.5:0.8b" in the Ollama library.
		lower := strings.ToLower(name)
		if i := strings.LastIndex(lower, "-"); i > 0 {
			candidate := lower[:i] + ":" + lower[i+1:]
			if ollamaLibraryHas(ctx, candidate) {
				suggestions = append(suggestions, candidate)
			}
		}
		for _, id := range ggufRepos(ctx, name) {
			suggestions = append(suggestions, "hf.co/"+id)
		}
	}
	msg := fmt.Sprintf("Ollama ne connaît pas « %s ». Utilise un nom de ollama.com/library (ex : qwen3:8b), ou un dépôt GGUF de Hugging Face précédé de hf.co/", model)
	if len(suggestions) > 0 {
		msg += ". Essaie : " + strings.Join(suggestions, ", ")
	}
	return errors.New(msg)
}

func ollamaLibraryHas(ctx context.Context, model string) bool {
	name, tag, _ := strings.Cut(model, ":")
	req, err := http.NewRequestWithContext(ctx, http.MethodHead,
		"https://registry.ollama.ai/v2/library/"+name+"/manifests/"+tag, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	res.Body.Close()
	return res.StatusCode == http.StatusOK
}

func lmStudioPresence(model string) Presence {
	info, _ := find(LMStudio)
	if !info.Installed {
		return PresenceUnknown
	}
	out, err := exec.Command(info.Path, "ls").Output()
	if err != nil {
		return PresenceUnknown
	}
	if strings.Contains(strings.ToLower(string(out)), strings.ToLower(model)) {
		return PresenceReady
	}
	return PresenceMissing
}

// lmStudioGet has no progress to read: lms only draws it for a terminal.
func lmStudioGet(ctx context.Context, model string) error {
	info, _ := find(LMStudio)
	if !info.Installed {
		return fmt.Errorf("LM Studio n'est pas installé. %s", info.Install)
	}
	cmd := exec.CommandContext(ctx, info.Path, "get", model)
	// No terminal to answer prompts: an ambiguous name fails instead of hanging.
	cmd.Stdin = nil
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("lms get : %s", strings.TrimSpace(string(out)))
	}
	return nil
}
