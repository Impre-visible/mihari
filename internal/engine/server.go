package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Impre-visible/mihari/internal/profiles"
)

// How long a model may take to load before Start gives up.
const loadTimeout = 15 * time.Minute

// Server is a running engine serving one profile's model.
type Server struct {
	// Base URL of the OpenAI-compatible API, without the /v1 suffix.
	URL string
	// Value for the "model" field of API requests.
	Model string

	cmd  *exec.Cmd
	done chan struct{}
	logs *tail
	// Runs on Stop, for engines Mihari does not own the process of.
	cleanup func()
}

// Start launches the engine of p and returns once its API answers.
func Start(ctx context.Context, p profiles.Profile) (*Server, error) {
	info, ok := find(Kind(p.Engine))
	switch {
	case !ok:
		return nil, fmt.Errorf("moteur inconnu : %q", p.Engine)
	case !info.Supported:
		return nil, fmt.Errorf("%s ne tourne pas sur cette machine", info.Label)
	case !info.Installed:
		return nil, fmt.Errorf("%s n'est pas installé. %s", info.Label, info.Install)
	case strings.TrimSpace(p.Model) == "":
		return nil, errors.New("aucun modèle renseigné dans le profil")
	}

	if info.Kind == LMStudio {
		return startLMStudio(ctx, info.Path, p)
	}

	port, err := freePort()
	if err != nil {
		return nil, err
	}
	s := &Server{URL: fmt.Sprintf("http://127.0.0.1:%d", port), logs: &tail{}}
	extra := strings.Fields(p.ExtraArgs)
	model := strings.TrimSpace(p.Model)

	switch info.Kind {
	case LlamaServer:
		// llama-server splits -c between its slots, so each job gets ContextPerJob.
		args := []string{
			"--host", "127.0.0.1", "--port", strconv.Itoa(port),
			"-np", strconv.Itoa(p.MaxJobs),
			"-c", strconv.Itoa(p.ContextPerJob * p.MaxJobs),
			"--alias", "mihari",
			// The model's chat template is what turns OpenAI tool definitions into its own format.
			"--jinja",
		}
		path := model
		if !isLocalPath(model) {
			if path, err = localGGUF(model); err != nil {
				return nil, err
			}
		}
		args = append(args, "-m", path)
		s.cmd = exec.Command(info.Path, append(args, extra...)...)
		s.Model = "mihari"
	case VLLMMLX:
		path := model
		if !isLocalPath(model) {
			if path, err = localSnapshot(model); err != nil {
				return nil, err
			}
		}
		args := []string{
			"serve", path,
			"--host", "127.0.0.1", "--port", strconv.Itoa(port),
			"--continuous-batching",
			"--max-num-seqs", strconv.Itoa(p.MaxJobs),
		}
		s.cmd = exec.Command(info.Path, append(args, extra...)...)
		s.Model = path
	case Ollama:
		if presence(Ollama, model) != PresenceReady {
			return nil, errNotDownloaded
		}
		// A separate `ollama serve`, so the user's own instance and settings stay untouched.
		s.cmd = exec.Command(info.Path, append([]string{"serve"}, extra...)...)
		s.cmd.Env = append(os.Environ(),
			"OLLAMA_HOST=127.0.0.1:"+strconv.Itoa(port),
			"OLLAMA_NUM_PARALLEL="+strconv.Itoa(p.MaxJobs),
			"OLLAMA_CONTEXT_LENGTH="+strconv.Itoa(p.ContextPerJob),
			"OLLAMA_MAX_LOADED_MODELS=1",
			"OLLAMA_KEEP_ALIVE=-1",
		)
		s.Model = model
	}

	s.cmd.Stdout = s.logs
	s.cmd.Stderr = s.logs
	setProcessGroup(s.cmd)
	if err := s.cmd.Start(); err != nil {
		return nil, fmt.Errorf("lancement de %s : %w", info.Label, err)
	}
	pid := s.cmd.Process.Pid
	track(pid, info.Path)
	s.done = make(chan struct{})
	go func() {
		_ = s.cmd.Wait()
		// Its children may outlive it; the group goes down with it.
		killGroup(pid)
		untrack(pid)
		close(s.done)
	}()

	if err := s.waitReady(ctx); err != nil {
		s.Stop()
		return nil, err
	}
	if info.Kind == Ollama {
		// Ollama loads lazily; an empty generate request loads the model now.
		if err := post(ctx, s.URL+"/api/generate", map[string]any{"model": model}, loadTimeout); err != nil {
			s.Stop()
			return nil, fmt.Errorf("chargement de %s : %w", model, err)
		}
	}
	return s, nil
}

// waitReady polls the API until it answers, the process exits, or loading times out.
func (s *Server) waitReady(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, loadTimeout)
	defer cancel()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.URL+"/v1/models", nil)
		if res, err := http.DefaultClient.Do(req); err == nil {
			res.Body.Close()
			if res.StatusCode == http.StatusOK {
				return nil
			}
		}
		select {
		case <-s.done:
			return fmt.Errorf("le moteur s'est arrêté au démarrage : %s", s.logs.last())
		case <-ctx.Done():
			return fmt.Errorf("le modèle ne répond pas après %s : %s", loadTimeout, s.logs.last())
		case <-ticker.C:
		}
	}
}

// Done is closed when the engine process exits.
func (s *Server) Done() <-chan struct{} {
	return s.done
}

// Logs returns the last lines the engine printed.
func (s *Server) Logs() string {
	return s.logs.last()
}

// Stop asks the engine to exit, and kills it if it has not after 10 seconds.
func (s *Server) Stop() {
	if s.cleanup != nil {
		s.cleanup()
	}
	if s.cmd == nil || s.cmd.Process == nil {
		return
	}
	select {
	case <-s.done:
		return
	default:
	}
	interrupt(s.cmd)
	select {
	case <-s.done:
	case <-time.After(10 * time.Second):
		kill(s.cmd)
		<-s.done
	}
}

var lmsPort = regexp.MustCompile(`port\D*(\d+)`)

// LM Studio runs a single server per machine, so Mihari loads its model there
// under its own identifier instead of starting a second instance.
func startLMStudio(ctx context.Context, lms string, p profiles.Profile) (*Server, error) {
	run := func(args ...string) (string, error) {
		out, err := exec.CommandContext(ctx, lms, args...).CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("lms %s : %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
		}
		return string(out), nil
	}

	if _, err := run("server", "start"); err != nil {
		return nil, err
	}
	status, err := run("server", "status")
	if err != nil {
		return nil, err
	}
	match := lmsPort.FindStringSubmatch(status)
	if match == nil {
		return nil, fmt.Errorf("port du serveur LM Studio introuvable dans : %s", strings.TrimSpace(status))
	}

	identifier := "mihari-" + p.ID
	args := []string{"load", strings.TrimSpace(p.Model), "--identifier", identifier,
		"--context-length", strconv.Itoa(p.ContextPerJob)}
	if _, err := run(append(args, strings.Fields(p.ExtraArgs)...)...); err != nil {
		return nil, err
	}

	done := make(chan struct{})
	var once sync.Once
	return &Server{
		URL:   "http://127.0.0.1:" + match[1],
		Model: identifier,
		done:  done,
		logs:  &tail{},
		cleanup: func() {
			once.Do(func() {
				_ = exec.Command(lms, "unload", identifier).Run()
				close(done)
			})
		},
	}, nil
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func post(ctx context.Context, url string, body any, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		var msg struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(res.Body).Decode(&msg)
		return fmt.Errorf("%s %s", res.Status, msg.Error)
	}
	return nil
}

// tail keeps the last few kilobytes an engine printed, for error messages.
type tail struct {
	mu  sync.Mutex
	buf []byte
}

const tailSize = 4096

func (t *tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > tailSize {
		t.buf = t.buf[len(t.buf)-tailSize:]
	}
	return len(p), nil
}

// last returns the final non-empty lines, short enough for a status line.
func (t *tail) last() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	lines := strings.Split(strings.TrimSpace(string(t.buf)), "\n")
	if len(lines) > 3 {
		lines = lines[len(lines)-3:]
	}
	return strings.Join(lines, " / ")
}
