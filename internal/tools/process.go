package tools

import (
	"fmt"
	"regexp"
)

// Background commands keep their pid and output here, in the container,
// which takes them away when the job ends.
const procDir = "/tmp/mihari/proc"

var processID = regexp.MustCompile(`^[a-z0-9-]{1,40}$`)

func procFiles(id string) (pid, log string, err error) {
	if !processID.MatchString(id) {
		return "", "", fmt.Errorf("identifiant de process invalide : %q (lettres minuscules, chiffres et tirets)", id)
	}
	return procDir + "/" + id + ".pid", procDir + "/" + id + ".log", nil
}
