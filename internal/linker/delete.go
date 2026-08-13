package linker

import (
	"fmt"
	"os"
)

// ValidateRemovalMappings checks that every recorded target is absent or still owned unless forced.
func (engine Engine) ValidateRemovalMappings(mappings []Mapping, force bool) error {
	for _, mapping := range mappings {
		if mapping.Target == "" || mapping.Source == "" {
			return fmt.Errorf("removal mapping source and target cannot be empty")
		}
		ownership, err := engine.InspectOwnership(mapping)
		if err != nil {
			return err
		}
		if ownership == OwnershipUnmanaged && !force {
			return fmt.Errorf("recorded target %s is no longer owned by source %s; use --force-targets to reclaim it", mapping.Target, mapping.Source)
		}
	}
	return nil
}

// ApplyRemovalMappings removes only the supplied recorded targets under the validated ownership policy.
func (engine Engine) ApplyRemovalMappings(mappings []Mapping, force bool) error {
	if err := engine.ValidateRemovalMappings(mappings, force); err != nil {
		return err
	}
	for _, mapping := range mappings {
		ownership, err := engine.InspectOwnership(mapping)
		if err != nil {
			return err
		}
		switch ownership {
		case OwnershipAbsent:
			continue
		case OwnershipOwned:
			if err := os.Remove(mapping.Target); err != nil {
				return err
			}
		case OwnershipUnmanaged:
			if err := removeTargetPath(mapping.Target); err != nil {
				return err
			}
		}
	}
	return nil
}
