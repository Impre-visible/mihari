package gitlab

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestUpload(t *testing.T) {
	const secret = "b66003fbdac0eb052b025af4b2cf99c4"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/projects/7/uploads/"+secret+"/my shot.png" || r.Header.Get("PRIVATE-TOKEN") != "tok" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte("PNGDATA"))
	}))
	defer srv.Close()
	base, _ := url.Parse(srv.URL)
	c := &Client{base: base, token: "tok", ProjectID: 7}
	ctx := context.Background()

	for _, link := range []string{
		"/uploads/" + secret + "/my%20shot.png",
		srv.URL + "/group/project/-/uploads/" + secret + "/my%20shot.png",
		srv.URL + "/group/project/uploads/" + secret + "/my%20shot.png",
	} {
		data, ok, err := c.Upload(ctx, link, 1<<20)
		if !ok || err != nil || string(data) != "PNGDATA" {
			t.Errorf("%s: %q, %v, %v", link, data, ok, err)
		}
	}
	// Anywhere else is not an attachment of this instance.
	for _, link := range []string{"https://example.com/uploads/" + secret + "/a.png", "https://example.com/image.png"} {
		if _, ok, _ := c.Upload(ctx, link, 1<<20); ok {
			t.Errorf("%s should not go through the API", link)
		}
	}
	if _, ok, err := c.Upload(ctx, "/uploads/"+secret+"/absent.png", 1<<20); !ok || err == nil {
		t.Errorf("a missing attachment should fail: %v", err)
	}
}
