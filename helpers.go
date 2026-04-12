package podcaster

import (
	"fmt"
	"os"
)

// writeFile writes data to a file, creating parent directories as needed.
func writeFile(path string, data []byte) error {
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}
