package engine

import (
	"reflect"
	"testing"
)

func TestPickGGUF(t *testing.T) {
	repo := []string{
		"README.md", "mmproj-Q8_0.gguf",
		"Qwen3-0.6B-BF16.gguf", "Qwen3-0.6B-Q4_K_M.gguf", "Qwen3-0.6B-Q8_0.gguf",
	}
	split := []string{
		"Q4_K_M/big-Q4_K_M-00002-of-00002.gguf", "Q4_K_M/big-Q4_K_M-00001-of-00002.gguf",
		"big-Q8_0.gguf",
	}
	cases := []struct {
		names []string
		quant string
		want  []string
	}{
		{repo, "", []string{"Qwen3-0.6B-Q4_K_M.gguf"}},
		{repo, "q8_0", []string{"Qwen3-0.6B-Q8_0.gguf"}},
		{split, "", []string{"Q4_K_M/big-Q4_K_M-00001-of-00002.gguf", "Q4_K_M/big-Q4_K_M-00002-of-00002.gguf"}},
		{[]string{"only-BF16.gguf"}, "", []string{"only-BF16.gguf"}},
	}
	for _, tc := range cases {
		got, err := pickGGUF(tc.names, tc.quant)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("pickGGUF(%v, %q) = %v, %v; want %v", tc.names, tc.quant, got, err, tc.want)
		}
	}
	if _, err := pickGGUF(repo, "IQ2_XS"); err == nil {
		t.Error("missing quant should fail")
	}
	if _, err := pickGGUF(split[:1], "Q4_K_M"); err == nil {
		t.Error("incomplete split should fail")
	}
}
