package devmode

import "errors"

// ErrDisabled is returned by development tools called from a production build.
var ErrDisabled = errors.New("outil réservé au mode développement")
