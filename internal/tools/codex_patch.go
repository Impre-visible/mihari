package tools

import (
	"fmt"
	"regexp"
	"strings"
)

var unifiedHeader = regexp.MustCompile(`^@@ -\d+(,\d+)? \+\d+(,\d+)? @@`)

// The patch format of Codex, also OpenCode's and Hermes's, which many models
// are trained to write:
//
//	*** Begin Patch
//	*** Add File: path      followed by "+" lines
//	*** Delete File: path
//	*** Update File: path   then optionally "*** Move to: path", then hunks
//	@@ optional line found first, then " ", "-" and "+" lines
//	*** End Patch
//
// This port follows OpenCode's (packages/util/src/patch.ts).

type codexOp struct {
	kind, path, moveTo string
	// Add: the new file's lines.
	lines  []string
	chunks []codexChunk
}

type codexChunk struct {
	context            string
	oldLines, newLines []string
	endOfFile          bool
}

func isCodexPatch(patch string) bool {
	return strings.HasPrefix(strings.TrimSpace(patch), "*** Begin Patch")
}

func codexBoundary(line string) bool {
	return line == "*** End Patch" || strings.HasPrefix(line, "*** Add File: ") ||
		strings.HasPrefix(line, "*** Delete File: ") || strings.HasPrefix(line, "*** Update File: ")
}

func parseCodexPatch(patch string) ([]codexOp, error) {
	lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(patch, "\r\n", "\n")), "\n")
	if strings.TrimSpace(lines[0]) != "*** Begin Patch" || strings.TrimSpace(lines[len(lines)-1]) != "*** End Patch" {
		return nil, fmt.Errorf("le patch doit commencer par *** Begin Patch et finir par *** End Patch")
	}
	lines = lines[1 : len(lines)-1]
	var ops []codexOp
	for i := 0; i < len(lines); {
		header := strings.TrimSpace(lines[i])
		switch {
		case header == "":
			i++
		case strings.HasPrefix(header, "*** Add File: "):
			op := codexOp{kind: "add", path: strings.TrimSpace(strings.TrimPrefix(header, "*** Add File: "))}
			for i++; i < len(lines) && !codexBoundary(strings.TrimSpace(lines[i])); i++ {
				switch {
				case strings.HasPrefix(lines[i], "+"):
					op.lines = append(op.lines, lines[i][1:])
				// A bare blank line can only mean a blank line.
				case strings.TrimSpace(lines[i]) == "":
					op.lines = append(op.lines, "")
				default:
					return nil, fmt.Errorf("ligne %q dans *** Add File: %s : chaque ligne du nouveau fichier commence par +, y compris les lignes vides (+ seul)", lines[i], op.path)
				}
			}
			ops = append(ops, op)
		case strings.HasPrefix(header, "*** Delete File: "):
			ops = append(ops, codexOp{kind: "delete", path: strings.TrimSpace(strings.TrimPrefix(header, "*** Delete File: "))})
			i++
		case strings.HasPrefix(header, "*** Update File: "):
			op := codexOp{kind: "update", path: strings.TrimSpace(strings.TrimPrefix(header, "*** Update File: "))}
			i++
			if i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "*** Move to: ") {
				op.moveTo = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[i]), "*** Move to: "))
				i++
			}
			var err error
			if op.chunks, i, err = parseCodexChunks(lines, i, op.path); err != nil {
				return nil, err
			}
			ops = append(ops, op)
		default:
			return nil, fmt.Errorf("ligne inattendue %q : attendu *** Add File, *** Delete File ou *** Update File", lines[i])
		}
	}
	if len(ops) == 0 {
		return nil, fmt.Errorf("patch vide")
	}
	return ops, nil
}

func parseCodexChunks(lines []string, i int, path string) ([]codexChunk, int, error) {
	var chunks []codexChunk
	for ; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimRight(line, " \t")
		if codexBoundary(trimmed) {
			break
		}
		switch {
		case trimmed == "*** End of File":
			if len(chunks) > 0 {
				chunks[len(chunks)-1].endOfFile = true
			}
			continue
		case trimmed == "@@" || strings.HasPrefix(trimmed, "@@ "):
			context := strings.TrimPrefix(strings.TrimPrefix(trimmed, "@@"), " ")
			// "@@ -1,3 +1,5 @@" is a unified diff's header, which says nothing here.
			if unifiedHeader.MatchString(trimmed) {
				context = ""
			}
			chunks = append(chunks, codexChunk{context: context})
			continue
		}
		if len(chunks) == 0 {
			chunks = append(chunks, codexChunk{})
		}
		c := &chunks[len(chunks)-1]
		switch {
		case line == "":
			c.oldLines, c.newLines = append(c.oldLines, ""), append(c.newLines, "")
		case line[0] == ' ':
			c.oldLines, c.newLines = append(c.oldLines, line[1:]), append(c.newLines, line[1:])
		case line[0] == '-':
			c.oldLines = append(c.oldLines, line[1:])
		case line[0] == '+':
			c.newLines = append(c.newLines, line[1:])
		default:
			return nil, 0, fmt.Errorf("ligne %q dans *** Update File: %s : chaque ligne commence par espace, - ou +", line, path)
		}
	}
	if len(chunks) == 0 {
		return nil, 0, fmt.Errorf("*** Update File: %s sans aucun changement", path)
	}
	return chunks, i, nil
}

// applyCodexChunks patches a file's content.
func applyCodexChunks(content, path string, chunks []codexChunk) (string, error) {
	trailing := strings.HasSuffix(content, "\n")
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	if content == "" {
		lines = nil
	}
	type replacement struct {
		start, remove int
		insert        []string
	}
	var reps []replacement
	at := 0
	for _, c := range chunks {
		if c.context != "" {
			found := seekLines(lines, []string{c.context}, at, false)
			if found < 0 {
				return "", fmt.Errorf("%s : la ligne de repère %q (après @@) n'est pas dans le fichier : mets-y une ligne du fichier, telle quelle, ou laisse @@ seul", path, c.context)
			}
			at = found + 1
		}
		if len(c.oldLines) == 0 {
			reps = append(reps, replacement{len(lines), 0, c.newLines})
			continue
		}
		oldLines, newLines := c.oldLines, c.newLines
		found := seekLines(lines, oldLines, at, c.endOfFile)
		// A trailing blank line in the hunk is often only the end of the patch.
		if found < 0 && oldLines[len(oldLines)-1] == "" {
			oldLines = oldLines[:len(oldLines)-1]
			if len(newLines) > 0 && newLines[len(newLines)-1] == "" {
				newLines = newLines[:len(newLines)-1]
			}
			found = seekLines(lines, oldLines, at, c.endOfFile)
		}
		if found < 0 {
			return "", fmt.Errorf("%s : lignes introuvables, relis le fichier :\n%s", path, strings.Join(c.oldLines, "\n"))
		}
		reps = append(reps, replacement{found, len(oldLines), newLines})
		at = found + len(oldLines)
	}
	// Applied from the end, so earlier positions stay right.
	for i := len(reps) - 1; i >= 0; i-- {
		r := reps[i]
		tail := append(append([]string{}, r.insert...), lines[r.start+r.remove:]...)
		lines = append(lines[:r.start], tail...)
	}
	out := strings.Join(lines, "\n")
	if trailing || content == "" {
		out += "\n"
	}
	return out, nil
}

// seekLines finds pattern in lines from start, exactly first, then more and
// more loosely: trailing spaces, all edge spaces, typographic punctuation.
func seekLines(lines, pattern []string, start int, eof bool) int {
	if len(pattern) == 0 {
		return -1
	}
	for _, same := range []func(a, b string) bool{
		func(a, b string) bool { return a == b },
		func(a, b string) bool { return strings.TrimRight(a, " \t") == strings.TrimRight(b, " \t") },
		func(a, b string) bool { return strings.TrimSpace(a) == strings.TrimSpace(b) },
		func(a, b string) bool {
			return typographic.Replace(strings.TrimSpace(a)) == typographic.Replace(strings.TrimSpace(b))
		},
	} {
		from, to := start, len(lines)-len(pattern)
		if eof {
			from = max(start, to)
		}
	offsets:
		for off := from; off <= to; off++ {
			for k, p := range pattern {
				if !same(lines[off+k], p) {
					continue offsets
				}
			}
			return off
		}
	}
	return -1
}

var typographic = strings.NewReplacer(
	"‘", "'", "’", "'", "‚", "'", "‛", "'",
	"“", `"`, "”", `"`, "„", `"`, "‟", `"`,
	"‐", "-", "‑", "-", "‒", "-", "–", "-", "—", "-", "―", "-", "−", "-",
	" ", " ", " ", " ", "　", " ",
)
