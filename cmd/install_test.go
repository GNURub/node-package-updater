package cmd

import (
	"testing"
)

func TestParseInstallArgs(t *testing.T) {
	tests := []struct {
		name          string
		dirFlag       string
		args          []string
		argsLenAtDash int
		wantDir       string
		wantArgs      []string
		wantErr       bool
	}{
		{
			name:          "path only",
			args:          []string{"./app"},
			argsLenAtDash: -1,
			wantDir:       "./app",
		},
		{
			name:          "dir flag only",
			dirFlag:       "./repo",
			args:          nil,
			argsLenAtDash: -1,
			wantDir:       "./repo",
		},
		{
			name:          "path and passthrough",
			args:          []string{"./repo", "--frozen-lockfile", "--prod"},
			argsLenAtDash: 1,
			wantDir:       "./repo",
			wantArgs:      []string{"--frozen-lockfile", "--prod"},
		},
		{
			name:          "passthrough only",
			args:          []string{"--frozen-lockfile"},
			argsLenAtDash: 0,
			wantArgs:      []string{"--frozen-lockfile"},
		},
		{
			name:          "rejects extra args without dash",
			args:          []string{"./repo", "--prod"},
			argsLenAtDash: -1,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installDir = tt.dirFlag
			t.Cleanup(func() { installDir = "" })

			gotDir, gotArgs, err := parseInstallArgs(dashCommand{dashIndex: tt.argsLenAtDash}, tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			if gotDir != tt.wantDir {
				t.Fatalf("dir = %q, want %q", gotDir, tt.wantDir)
			}
			if len(gotArgs) != len(tt.wantArgs) {
				t.Fatalf("args = %v, want %v", gotArgs, tt.wantArgs)
			}
			for i := range gotArgs {
				if gotArgs[i] != tt.wantArgs[i] {
					t.Fatalf("args = %v, want %v", gotArgs, tt.wantArgs)
				}
			}
		})
	}
}

type dashCommand struct {
	dashIndex int
}

func (d dashCommand) ArgsLenAtDash() int {
	return d.dashIndex
}
