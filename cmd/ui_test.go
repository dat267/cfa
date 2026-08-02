package cmd

import (
	"os"
	"testing"
)

func TestConfirmAction(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"yes short", "y\n", true},
		{"yes word", "yes\n", true},
		{"yes uppercase", "Y\n", true},
		{"no", "n\n", false},
		{"bare enter defaults to no", "\n", false},
		{"eof defaults to no", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatalf("pipe: %v", err)
			}
			old := os.Stdin
			os.Stdin = r
			if _, err := w.WriteString(tt.input); err != nil {
				t.Fatalf("write: %v", err)
			}
			_ = w.Close()

			got, err := confirmAction("Proceed?")

			os.Stdin = old
			_ = r.Close()

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("confirmAction(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
