package bundle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/opencontainers/runtime-spec/specs-go"
)

func (b *Bundle) loadSpec() error {
	specPath := filepath.Join(b.Dir, specFileName)
	specFile, err := os.Open(specPath)
	if err != nil {
		return err
	}
	defer specFile.Close()

	var spec specs.Spec
	decoder := json.NewDecoder(specFile)
	err = decoder.Decode(&spec)
	if err != nil {
		return fmt.Errorf("Fail to parse OCI spec: %w", err)
	}

	b.spec = &spec
	b.logger.Debug().Interface("Spec", b.spec).Msg("OCI Spec loaded")
	return nil
}
