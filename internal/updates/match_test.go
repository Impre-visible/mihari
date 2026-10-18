package updates

import (
	"testing"

	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

func TestMatchAsset(t *testing.T) {
	// What the release workflow publishes.
	assets := []github.ReleaseAsset{
		{Name: "Mihari-0.2.0-macos-arm64.dmg"}, {Name: "mihari-0.2.0-darwin-arm64.zip"},
		{Name: "Mihari-0.2.0-linux-x86_64.AppImage"}, {Name: "mihari_0.2.0_amd64.deb"}, {Name: "mihari-0.2.0-x86_64.pkg.tar.zst"},
		{Name: "Mihari-0.2.0-windows-amd64-setup.exe"}, {Name: "mihari-0.2.0-windows-amd64.zip"}, {Name: "SHA256SUMS"},
	}
	for _, c := range []struct {
		method   Method
		platform string
		want     string
	}{
		{MethodInPlace, "darwin", "mihari-0.2.0-darwin-arm64.zip"},
		{MethodInPlace, "windows", "mihari-0.2.0-windows-amd64.zip"},
		{MethodAppImage, "linux", "Mihari-0.2.0-linux-x86_64.AppImage"},
		{MethodPacman, "linux", "mihari-0.2.0-x86_64.pkg.tar.zst"},
		{MethodApt, "linux", "mihari_0.2.0_amd64.deb"},
	} {
		i := matcherFor(c.method)(updater.CheckRequest{Platform: c.platform}, assets)
		if i < 0 || assets[i].Name != c.want {
			t.Errorf("%s on %s: got %d, want %s", c.method, c.platform, i, c.want)
		}
	}
	if matcherFor(MethodManual)(updater.CheckRequest{Platform: "freebsd"}, assets) != -1 {
		t.Error("an unknown platform must get nothing")
	}
}
