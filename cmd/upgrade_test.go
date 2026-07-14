package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestUpgradeCommand_RefusesOnThisFork(t *testing.T) {
	cmd := newUpgradeCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("upgrade should refuse on this fork")
	}

	msg := err.Error() + stdout.String() + stderr.String()
	const want = "not supported on this fork yet"
	if !strings.Contains(msg, want) {
		t.Fatalf("expected refusal mentioning %q; got error=%v stdout=%q stderr=%q",
			want, err, stdout.String(), stderr.String())
	}
}

func TestUpgradeCommand_HelpDoesNotPointAtIksnae(t *testing.T) {
	cmd := newUpgradeCmd()
	cmd.SetArgs([]string{"--help"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("upgrade --help: %v", err)
	}

	out := stdout.String() + stderr.String()
	if strings.Contains(out, "iksnae") {
		t.Fatalf("upgrade help must not mention iksnae; got:\n%s", out)
	}
}
