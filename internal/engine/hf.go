package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// Files Mihari downloads from Hugging Face live under the user cache, one
// folder per repository, so engines get a local path and never download.
func modelsDir() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "mihari", "models"), nil
}

// hfRef is "org/repo", optionally followed by ":QUANT" to pick a GGUF file.
type hfRef struct {
	Repo  string
	Quant string
}

func parseHFRef(model string) (hfRef, error) {
	repo, quant, _ := strings.Cut(strings.TrimSpace(model), ":")
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return hfRef{}, fmt.Errorf("%q n'est pas un dépôt Hugging Face (org/depot)", model)
	}
	return hfRef{Repo: repo, Quant: quant}, nil
}

func (r hfRef) dir() (string, error) {
	base, err := modelsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "hf", filepath.FromSlash(r.Repo)), nil
}

type hfFile struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
	Type string `json:"type"`
}

func hfList(ctx context.Context, repo string) ([]hfFile, error) {
	endpoint := "https://huggingface.co/api/models/" + repo + "/tree/main?recursive=true"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Hugging Face injoignable : %w", err)
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusUnauthorized:
		return nil, fmt.Errorf("dépôt %s introuvable sur Hugging Face (ou privé)", repo)
	case res.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("Hugging Face : %s", res.Status)
	}
	var all []hfFile
	if err := json.NewDecoder(res.Body).Decode(&all); err != nil {
		return nil, err
	}
	files := all[:0]
	for _, f := range all {
		if f.Type == "file" {
			files = append(files, f)
		}
	}
	return files, nil
}

var splitGGUF = regexp.MustCompile(`(?i)-(\d{5})-of-(\d{5})\.gguf$`)

// pickGGUF chooses the weights among a repository's file names, the same way
// online before downloading and offline before starting llama-server. A split
// model returns all its parts, the first one first.
func pickGGUF(names []string, quant string) ([]string, error) {
	var ggufs []string
	for _, n := range names {
		lower := strings.ToLower(n)
		// Vision projectors and multi-token-prediction drafts ride along with
		// the weights but are not the model.
		if strings.HasSuffix(lower, ".gguf") && !strings.Contains(lower, "mmproj") &&
			!strings.HasPrefix(path.Base(lower), "mtp-") && !strings.Contains(lower, "mtp/") {
			ggufs = append(ggufs, n)
		}
	}
	sort.Strings(ggufs)
	if len(ggufs) == 0 {
		return nil, errNoGGUF
	}

	// A quant is a whole token of the name: Q6_K must not pick Q6_K_L.
	matching := func(q string) []string {
		if q == "" {
			return ggufs
		}
		token := regexp.MustCompile(`(?i)(^|[-_./])` + regexp.QuoteMeta(q) + `([-_.]|$)`)
		var out []string
		for _, n := range ggufs {
			if token.MatchString(n) {
				out = append(out, n)
			}
		}
		return out
	}
	var candidates []string
	if quant != "" {
		if candidates = matching(quant); len(candidates) == 0 {
			return nil, fmt.Errorf("aucun fichier .gguf en %s dans ce dépôt", quant)
		}
	} else {
		// Q4_K_M is llama.cpp's own default for -hf; then the best quality that is still common.
		for _, q := range []string{"Q4_K_M", "Q8_0", ""} {
			if candidates = matching(q); len(candidates) > 0 {
				break
			}
		}
	}

	first := candidates[0]
	m := splitGGUF.FindStringSubmatch(first)
	if m == nil {
		return []string{first}, nil
	}
	prefix := first[:len(first)-len(m[0])]
	count, _ := strconv.Atoi(m[2])
	var parts []string
	for _, n := range candidates {
		if strings.HasPrefix(n, prefix) && splitGGUF.MatchString(n) {
			parts = append(parts, n)
		}
	}
	if len(parts) != count {
		return nil, fmt.Errorf("modèle découpé incomplet : %d parties sur %d", len(parts), count)
	}
	return parts, nil
}

// localGGUF returns the downloaded file llama-server should load for model.
func localGGUF(model string) (string, error) {
	ref, err := parseHFRef(model)
	if err != nil {
		return "", err
	}
	dir, err := ref.dir()
	if err != nil {
		return "", err
	}
	var names []string
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(dir, path)
			names = append(names, filepath.ToSlash(rel))
		}
		return nil
	})
	parts, err := pickGGUF(names, ref.Quant)
	if err != nil {
		return "", errNotDownloaded
	}
	return filepath.Join(dir, filepath.FromSlash(parts[0])), nil
}

// A snapshot is complete once this marker exists, since a half-downloaded
// MLX repository still has a config.json.
const completeMarker = ".mihari-complete"

func localSnapshot(model string) (string, error) {
	ref, err := parseHFRef(model)
	if err != nil {
		return "", err
	}
	dir, err := ref.dir()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(dir, completeMarker)); err != nil {
		return "", errNotDownloaded
	}
	return dir, nil
}

var errNoGGUF = errors.New("aucun fichier .gguf dans ce dépôt")

// noGGUFError points to GGUF conversions of a repository that only has
// safetensors, which is what an official model repository usually holds.
func noGGUFError(ctx context.Context, repo string) error {
	_, name, _ := strings.Cut(repo, "/")
	if ids := ggufRepos(ctx, name); len(ids) > 0 {
		return fmt.Errorf("%s n'a pas de fichier .gguf, llama-server ne peut pas le lire. Versions GGUF : %s",
			repo, strings.Join(ids, ", "))
	}
	return fmt.Errorf("%s n'a pas de fichier .gguf, llama-server ne peut pas le lire. Cherche une version « %s-GGUF »", repo, name)
}

// ggufRepos returns the most downloaded GGUF conversions of a model name.
func ggufRepos(ctx context.Context, name string) []string {
	query := url.Values{"search": {name + "-GGUF"}, "sort": {"downloads"}, "limit": {"3"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://huggingface.co/api/models?"+query.Encode(), nil)
	if err != nil {
		return nil
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil
	}
	defer res.Body.Close()
	var found []struct {
		ID string `json:"id"`
	}
	if json.NewDecoder(res.Body).Decode(&found) != nil {
		return nil
	}
	ids := make([]string, len(found))
	for i, f := range found {
		ids[i] = f.ID
	}
	return ids
}

var errNotDownloaded = errors.New("modèle pas encore téléchargé : clique sur Télécharger dans Profils")

// downloadHF fetches either the GGUF weights (onlyGGUF) or the whole repository.
func downloadHF(ctx context.Context, model string, onlyGGUF bool, progress func(done, total int64)) error {
	ref, err := parseHFRef(model)
	if err != nil {
		return err
	}
	dir, err := ref.dir()
	if err != nil {
		return err
	}
	files, err := hfList(ctx, ref.Repo)
	if err != nil {
		return err
	}

	if onlyGGUF {
		names := make([]string, len(files))
		for i, f := range files {
			names[i] = f.Path
		}
		parts, err := pickGGUF(names, ref.Quant)
		if errors.Is(err, errNoGGUF) {
			return noGGUFError(ctx, ref.Repo)
		}
		if err != nil {
			return err
		}
		keep := map[string]bool{}
		for _, p := range parts {
			keep[p] = true
		}
		selected := files[:0]
		for _, f := range files {
			if keep[f.Path] {
				selected = append(selected, f)
			}
		}
		files = selected
	}

	var total, done int64
	for _, f := range files {
		total += f.Size
	}
	progress(0, total)
	for _, f := range files {
		dest := filepath.Join(dir, filepath.FromSlash(f.Path))
		if info, err := os.Stat(dest); err == nil && info.Size() == f.Size {
			done += f.Size
			progress(done, total)
			continue
		}
		base := done
		err := downloadFile(ctx, "https://huggingface.co/"+ref.Repo+"/resolve/main/"+escapePath(f.Path), dest,
			func(n int64) { progress(base+n, total) })
		if err != nil {
			return fmt.Errorf("%s : %w", f.Path, err)
		}
		done += f.Size
	}
	if !onlyGGUF {
		return os.WriteFile(filepath.Join(dir, completeMarker), nil, 0o644)
	}
	return nil
}

func escapePath(p string) string {
	segments := strings.Split(p, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return strings.Join(segments, "/")
}

// downloadFile writes url to dest through a .part file, resuming one left by
// an interrupted download.
func downloadFile(ctx context.Context, url, dest string, progress func(int64)) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	part := dest + ".part"
	var offset int64
	if info, err := os.Stat(part); err == nil {
		offset = info.Size()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	flags := os.O_CREATE | os.O_WRONLY
	switch res.StatusCode {
	case http.StatusPartialContent:
		flags |= os.O_APPEND
	case http.StatusOK:
		offset = 0
		flags |= os.O_TRUNC
	default:
		return fmt.Errorf("téléchargement : %s", res.Status)
	}
	out, err := os.OpenFile(part, flags, 0o644)
	if err != nil {
		return err
	}
	written := offset
	buf := make([]byte, 1<<20)
	for {
		n, readErr := res.Body.Read(buf)
		if n > 0 {
			if _, err := out.Write(buf[:n]); err != nil {
				out.Close()
				return err
			}
			written += int64(n)
			progress(written)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			out.Close()
			return readErr
		}
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(part, dest)
}

// RemoteSize returns how many bytes downloading model for kind would take,
// read from Hugging Face without downloading anything.
func RemoteSize(ctx context.Context, kind Kind, model string) (int64, error) {
	ref, err := parseHFRef(model)
	if err != nil {
		return 0, err
	}
	files, err := hfList(ctx, ref.Repo)
	if err != nil {
		return 0, err
	}
	var total int64
	switch kind {
	case LlamaServer:
		names := make([]string, len(files))
		for i, f := range files {
			names[i] = f.Path
		}
		parts, err := pickGGUF(names, ref.Quant)
		if err != nil {
			return 0, err
		}
		for _, f := range files {
			if slices.Contains(parts, f.Path) {
				total += f.Size
			}
		}
	default:
		// MLX weights; the rest of the repository is small.
		for _, f := range files {
			if strings.HasSuffix(f.Path, ".safetensors") {
				total += f.Size
			}
		}
	}
	if total == 0 {
		return 0, errors.New("aucun fichier de poids trouvé")
	}
	return total, nil
}
