package migrate

import (
	"fmt"
	"os/exec"
)

func findBinary(name, path string) (string, error) {
	if path != "" {
		if _, err := exec.LookPath(path); err != nil {
			return "", fmt.Errorf("%s not found at %q: %w", name, path, err)
		}

		return path, nil
	}

	resolved, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s not found in PATH; pass --%s /path/to/%s", name, name, name)
	}

	return resolved, nil
}
