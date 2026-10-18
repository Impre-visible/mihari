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
		// bartowski ships Q6_K next to Q6_K_L.
		{[]string{"m-Q6_K_L.gguf", "m-Q6_K.gguf"}, "Q6_K", []string{"m-Q6_K.gguf"}},
		// Splits in a folder named after the quant.
		{[]string{"Q8_0/m-Q8_0-00002-of-00002.gguf", "Q8_0/m-Q8_0-00001-of-00002.gguf", "m-Q4_K_M.gguf"}, "Q8_0",
			[]string{"Q8_0/m-Q8_0-00001-of-00002.gguf", "Q8_0/m-Q8_0-00002-of-00002.gguf"}},
	}
	for _, tc := range cases {
		got, err := pickGGUF(tc.names, tc.quant)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("pickGGUF(%v, %q) = %v, %v; want %v", tc.names, tc.quant, got, err, tc.want)
		}
	}
	// Draft models for speculative decoding are not the model.
	if got, err := pickGGUF([]string{"MTP/mtp-m-Q4_K_M.gguf", "m-UD-Q4_K_XL.gguf"}, "Q4_K_M"); err == nil {
		t.Errorf("an MTP draft was picked as the model: %v", got)
	}
	if _, err := pickGGUF(repo, "IQ2_XS"); err == nil {
		t.Error("missing quant should fail")
	}
	if _, err := pickGGUF(split[:1], "Q4_K_M"); err == nil {
		t.Error("incomplete split should fail")
	}
}
