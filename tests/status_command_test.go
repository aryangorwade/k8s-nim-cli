package tests

import (
	"testing"

	statuscmd "k8s-nim-operator-cli/pkg/cmd/status"
)

func Test_NewStatusCommand_Wiring(t *testing.T) {
	streams, _, _, _ := genericTestIOStreams()
	cmd := statuscmd.NewStatusCommand(nil, streams)
	if cmd.Use != "status" {
		t.Fatalf("Use = %q", cmd.Use)
	}
	// ensure subcommands are present
	subs := cmd.Commands()
	if len(subs) < 2 {
		t.Fatalf("expected subcommands for nimcache and nimservice, got %d", len(subs))
	}
}
