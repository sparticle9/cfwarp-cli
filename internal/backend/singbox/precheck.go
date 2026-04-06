package singbox

import (
	"context"
	"fmt"
	"os/exec"
)

const binaryName = "sing-box"

// ValidatePrereqs checks that the sing-box binary is available in PATH.
func ValidatePrereqs(_ context.Context) error {
	path, err := exec.LookPath(binaryName)
	if err != nil {
		return fmt.Errorf("%s binary not found in PATH: install sing-box >= 1.13 (https://sing-box.sagernet.org)", binaryName)
	}
	_ = path
	return nil
}
