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
			name:     "secret without quotes",
			text:     `API_KEY = aB3dE5fG7hJ9kL1mN2oP4qR6sT8uV0wX`,
			minFound: 1,
		},
		{
			name:     "secret with double quotes",
			text:     `API_KEY = "aB3dE5fG7hJ9kL1mN2oP4qR6sT8uV0wX"`,
			minFound: 1,
		},
		{
			name:     "secret with single quotes",
			text:     `API_KEY = 'aB3dE5fG7hJ9kL1mN2oP4qR6sT8uV0wX'`,
			minFound: 1,
		},
		{
			name:     "secret with special chars (Base64)",
			text:     `TOKEN = "aB3dE5fG7hJ9kL1mN2oP4qR6sT8uV0wX+/="`,
			minFound: 1,
		},
		{
			name:     "secret with colon separator",
			text:     `password: aB3dE5fG7hJ9kL1mN2oP4qR6sT8uV0wX`,
			minFound: 1,
		},
		{
			name:     "secret without spaces",
			text:     `SECRET_KEY=aB3dE5fG7hJ9kL1mN2oP4qR6sT8uV0wX`,
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

	files := map[string]string{
		"test1.env": `API_KEY=aB3dE5fG7hJ9kL1mN2oP4qR6sT8uV0wX`,
		"test2.env": `API_KEY = "aB3dE5fG7hJ9kL1mN2oP4qR6sT8uV0wX"`,
		"test3.env": `TOKEN = 'aB3dE5fG7hJ9kL1mN2oP4qR6sT8uV0wX'`,
		"test4.env": `password: aB3dE5fG7hJ9kL1mN2oP4qR6sT8uV0wX`,
		"test5.txt": `SECRET_KEY=aB3dE5fG7hJ9kL1mN2oP4qR6sT8uV0wX`,
	}

	for filename, content := range files {
		path := filepath.Join(tmpDir, filename)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	candidates := analyzer.AnalyzeRepository(
		tmpDir,
		[]string{".env", ".txt"},
		1024,
		[]string{},
	)

	if len(candidates) < 5 {
		t.Errorf("expected at least 5 candidates, got %d", len(candidates))
	}
}

func BenchmarkAnalyzeText(b *testing.B) {
	analyzer := NewAnalyzer(8, 256, 4.5, 0.6)
	text := "API_KEY = \"aB3dE5fG7hJ9kL1mN2oP4qR6sT8uV0wX\"\n"

	for i := 0; i < 1000; i++ {
		text += "Normal line without secrets\n"
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.AnalyzeText(text, "bench.txt", true)
	}
}
