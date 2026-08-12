// Command fetch-trusted-root fetches the current public-good Sigstore trusted
// root via TUF and writes it as vendored JSON, to be embedded in the binary.
// Run it with `mise run roots:update` to refresh the pinned root.
package main

import (
	"fmt"
	"os"

	"github.com/sigstore/sigstore-go/pkg/root"
)

// outPerm is the mode for the vendored public root (world-readable public data).
const outPerm = 0o644

func main() {
	out := "internal/verify/roots/public_good_trusted_root.json"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}

	tr, err := root.FetchTrustedRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "fetch trusted root:", err)
		os.Exit(1)
	}
	data, err := tr.MarshalJSON()
	if err != nil {
		fmt.Fprintln(os.Stderr, "marshal:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(out, data, outPerm); err != nil { //nolint:gosec // vendored public data, world-readable is fine
		fmt.Fprintln(os.Stderr, "write:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%d bytes)\n", out, len(data))
}
