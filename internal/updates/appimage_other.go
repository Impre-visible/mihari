//go:build !linux

package updates

import (
	"context"
	"errors"

	"github.com/wailsapp/wails/v3/pkg/updater"
)

// Only Linux runs from an AppImage.
func (s *Service) replaceAppImage(context.Context, *updater.Release) error {
	return errors.New("pas une AppImage")
}

func packageMethod() Method { return MethodManual }

func (s *Service) installPackage(context.Context, *updater.Release, Method) error {
	return errors.New("pas un paquet Linux")
}
