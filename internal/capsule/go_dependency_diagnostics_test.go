package capsule

import (
	"strings"
	"testing"
)

func TestInspectGoListOutput(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantState GoDependencyState
		wantErr   bool
	}{
		{
			name: "dependencies ready",
			input: `{"ImportPath":"fmt"}
{"ImportPath":"example.com/demo","Imports":["fmt"]}`,
			wantState: GoDependenciesReady,
		},
		{
			name: "external module unavailable",
			input: `{
				"ImportPath": "github.com/google/uuid",
				"Incomplete": true,
				"Error": {
					"Err": "module lookup disabled by GOPROXY=off"
				}
			}`,
			wantState: GoDependenciesUnknown,
		},
		{
			name: "invalid source import",
			input: `{
				"ImportPath": "example.com/demo",
				"Incomplete": true,
				"DepsErrors": [
					{
						"Err": "no required module provides package example.com/missing"
					}
				]
			}`,
			wantState: GoDependenciesUnknown,
		},
		{
			name:      "empty output",
			input:     "",
			wantState: GoDependenciesUnknown,
		},
		{
			name:    "invalid JSON",
			input:   `{"ImportPath":`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := InspectGoListOutput(strings.NewReader(tt.input))

			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr = %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if got.State != tt.wantState {
				t.Fatalf(
					"state = %q, want %q",
					got.State,
					tt.wantState,
				)
			}

			if err := got.Validate(); err != nil {
				t.Fatalf("invalid status: %v", err)
			}

			if tt.wantState == GoDependenciesUnknown && got.Reason == "" {
				t.Fatal("unknown state requires an explanation")
			}
		})
	}
}

func TestInspectGoListOutputTooLarge(t *testing.T) {
	input := `{"ImportPath":"example.com/demo","Extra":"` +
		strings.Repeat("x", int(MaxGoDependencyDiagnosticsSize)) +
		`"}`

	_, err := InspectGoListOutput(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for oversized go list output")
	}
}

func TestInspectGoListOutputTruncatedJSON(t *testing.T) {
	input := `{"ImportPath":"fmt"}
{"ImportPath":"example.com/demo","Incomplete":`

	_, err := InspectGoListOutput(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for truncated JSON")
	}
}

func TestInspectGoListOutputIncompleteWithoutError(t *testing.T) {
	input := `{"ImportPath":"example.com/demo","Incomplete":true}`

	got, err := InspectGoListOutput(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}

	if got.State != GoDependenciesUnknown {
		t.Fatalf(
			"state = %q, want %q",
			got.State,
			GoDependenciesUnknown,
		)
	}
}
