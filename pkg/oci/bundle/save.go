package bundle

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
)

func (b *Bundle) SaveSpec() error {
	specRaw, err := json.Marshal(b.spec)
	if err != nil {
		return fmt.Errorf("Failed to marshal OCI Spec: %v", err)
	}

	specPath := filepath.Join(b.Dir, specFileName)
	err = ioutil.WriteFile(specPath, specRaw, 0644)
	if err != nil {
		return err
	}
	b.logger.Debug().Bytes("UpdatedSpec", specRaw).Msg("OCI Spec updated")

	return nil
}
