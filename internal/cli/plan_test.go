package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRetiredTailoringCommandsReturnMigrationDiagnosticWithoutRuntime(t *testing.T) {
	tests := [][]string{
		{"policy", "init", "--catalog", "catalog.json", "--effect", "read", "--", "item", "list"},
		{"policy", "validate", "--catalog", "catalog.json", "--policy", "policy.yaml"},
		{"plan", "preview", "--config", "policy.yaml", "--", "source", "item", "list"},
		{"run", "--config", "policy.yaml", "--", "source", "item", "list"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args[:2], "_"), func(t *testing.T) {
			var stderr bytes.Buffer
			cli := New(strings.NewReader(""), &bytes.Buffer{}, &stderr)
			code := cli.RunContext(context.Background(), append([]string{"--error-format=json"}, args...))
			if code == ExitOK || !strings.Contains(stderr.String(), `"code":"legacy_tailoring_schema"`) {
				t.Fatalf("code=%d stderr=%s", code, stderr.String())
			}
		})
	}
}
