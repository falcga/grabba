package analyzer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeText(t *testing.T) {
	analyzer := NewAnalyzer(8, 256, 4.5, 0.6)

	tests := []struct {
		name     string
		text     string
		minFound int
	}{
		{
			name:     "contains secret",
			text:     `SECRET_KEY = "f7g8h9j0k1l2m3n4o5p6q7r8s9t0u1v2"`,
			minFound: 1,
		},
		{
			name:     "no secret",
			text:     "Hello world, this is a normal text",
			minFound: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidates := analyzer.AnalyzeText(tt.text, "test.txt", true)
			if len(candidates) < tt.minFound {
				t.Errorf("expected at least %d candidates, got %d", tt.minFound, len(candidates))
			}
		})
	}
}

func TestAnalyzeRepository(t *testing.T) {
	analyzer := NewAnalyzer(8, 256, 4.5, 0.6)

	tmpDir := t.TempDir()

	secretFile := filepath.Join(tmpDir, "test.env")
	err := os.WriteFile(secretFile, []byte("API_KEY=abc123def456ghi789jkl"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	candidates := analyzer.AnalyzeRepository(
		tmpDir,
		[]string{".env"},
		1024,
		[]string{},
	)

	if len(candidates) == 0 {
		t.Error("expected to find secrets in test repository")
	}
}

func BenchmarkAnalyzeText(b *testing.B) {
	analyzer := NewAnalyzer(8, 256, 4.5, 0.6)
	text := "SECRET_KEY = \"f7g8h9j0k1l2m3n4o5p6q7r8s9t0u1v2\"\n"

	for i := 0; i < 1000; i++ {
		text += "Normal line without secrets\n"
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.AnalyzeText(text, "bench.txt", true)
	}
}
