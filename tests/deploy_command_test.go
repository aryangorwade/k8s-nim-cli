package tests

import (
	"testing"

	deploycmd "k8s-nim-operator-cli/pkg/cmd/deploy"
)

func Test_NewDeployCommand_Wiring(t *testing.T) {
	streams, _, _, _ := genericTestIOStreams()
	cmd := deploycmd.NewDeployCommand(nil, streams)
	if cmd.Use != "deploy" {
		t.Fatalf("Use = %q", cmd.Use)
	}
	if len(cmd.Aliases) == 0 || cmd.Aliases[0] != "create" {
		t.Fatalf("aliases = %v", cmd.Aliases)
	}
	// ensure subcommands are present
	if len(cmd.Commands()) < 2 {
		t.Fatalf("expected NIMCache and NIMService subcommands")
	}
}