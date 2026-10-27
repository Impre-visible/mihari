package agent

import "fmt"

// After this many repeated calls, then as many more each time, the model is
// told it is going round in circles.
const repeatsBeforeNudge = 3

// pastCall is the latest run of a call: when, what it gave, and how many
// changes had been made once it ran.
type pastCall struct {
	turn   int
	result string
	writes int
}

// repeats spots a call made again while nothing changed in between: models,
// small ones above all, read the same file or post the same comment again
// instead of moving on.
type repeats struct {
	calls map[string]pastCall
	// Successful calls of tools that change something, so far.
	writes int
	count  int
	nudged int
}

// again returns the earlier run of the call, and whether nothing was changed
// since: no other write for a write, no write at all for a read.
func (r *repeats) again(name, args string) (pastCall, bool) {
	p, ok := r.calls[signature(name, args)]
	return p, ok && p.writes == r.writes
}

// record notes a call that ran, and counts it when it repeated one.
func (r *repeats) record(name, args string, turn int, result string, wrote bool) {
	sig := signature(name, args)
	if p, ok := r.calls[sig]; ok && p.writes == r.writes && p.result == result {
		r.count++
	}
	if wrote {
		r.writes++
	}
	r.calls[sig] = pastCall{turn: turn, result: result, writes: r.writes}
}

// skipped counts a write call refused as a repeat.
func (r *repeats) skipped() { r.count++ }

// nudge returns what to tell the model when it has repeated itself enough,
// "" otherwise; it says it once per batch of repeats.
func (r *repeats) nudge() string {
	if r.count < (r.nudged+1)*repeatsBeforeNudge {
		return ""
	}
	r.nudged++
	return fmt.Sprintf("Tu as refait %d fois des appels identiques sans que rien ne change : tu tournes en rond. "+
		"Fais le point sur ce que tu sais déjà, change d'approche (autre fichier, autre recherche, subagent), "+
		"ou appelle finish si le travail est fait.", r.count)
}

// clip shortens s to n characters at most.
func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
