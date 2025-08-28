package tests

import (
	"bytes"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// Shared IOStreams helper for tests in this package
func genericTestIOStreams() (s genericclioptions.IOStreams, in *bytes.Buffer, out *bytes.Buffer, errOut *bytes.Buffer) {
	in = &bytes.Buffer{}
	out = &bytes.Buffer{}
	errOut = &bytes.Buffer{}
	return genericclioptions.IOStreams{In: in, Out: out, ErrOut: errOut}, in, out, errOut
}
