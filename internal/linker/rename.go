package linker

import (
	"fmt"
	"os"
)

// MappingRewrite describes one active managed link whose source or target changes.
type MappingRewrite struct {
	Previous Mapping
	Next     Mapping
}

// ValidateRenameMappings checks ownership for every affected target before a rename commits.
func (engine Engine) ValidateRenameMappings(rewrites []MappingRewrite, force bool) error {
	for _, rewrite := range rewrites {
		if rewrite.Previous.Target == "" || rewrite.Next.Target == "" {
			return fmt.Errorf("rename mapping target cannot be empty")
		}
		if err := validateRenameTarget(engine, rewrite.Previous, force); err != nil {
			return err
		}
		if rewrite.Next.Target != rewrite.Previous.Target {
			if err := validateRenameDestination(engine, rewrite.Next, force); err != nil {
				return err
			}
		}
	}
	return nil
}

// ApplyRenameMappings replaces affected links after repository paths have moved.
func (engine Engine) ApplyRenameMappings(rewrites []MappingRewrite, force bool) error {
	if err := engine.ValidateRenameMappings(rewrites, force); err != nil {
		return err
	}
	for _, rewrite := range rewrites {
		if rewrite.Previous.Target != rewrite.Next.Target {
			if err := removeRenameTarget(engine, rewrite.Previous, force); err != nil {
				return err
			}
			if err := removeRenameDestination(engine, rewrite.Next, force); err != nil {
				return err
			}
		} else if err := removeRenameTarget(engine, rewrite.Previous, force); err != nil {
			return err
		}
		if err := createOwnedSymlink(rewrite.Next); err != nil {
			return err
		}
	}
	return nil
}

// validateRenameTarget requires an affected recorded target to remain owned unless forced.
// The spec treats an absent target like drift: rename fails without --force-targets.
func validateRenameTarget(engine Engine, mapping Mapping, force bool) error {
	ownership, err := engine.InspectOwnership(mapping)
	if err != nil {
		return err
	}
	if ownership == OwnershipOwned {
		return nil
	}
	if force && (ownership == OwnershipAbsent || ownership == OwnershipUnmanaged) {
		return nil
	}
	return fmt.Errorf("recorded target %s is %s; use --force-targets to reclaim it", mapping.Target, ownership)
}

// validateRenameDestination checks a newly derived target before it is installed.
func validateRenameDestination(engine Engine, mapping Mapping, force bool) error {
	ownership, err := engine.InspectOwnership(mapping)
	if err != nil {
		return err
	}
	if ownership == OwnershipAbsent || ownership == OwnershipOwned {
		return nil
	}
	if force {
		return nil
	}
	return fmt.Errorf("rename destination target %s is unmanaged; use --force-targets to reclaim it", mapping.Target)
}

// removeRenameTarget removes the old recorded target while respecting ownership policy.
// An absent recorded target still blocks recreation unless forced.
func removeRenameTarget(engine Engine, mapping Mapping, force bool) error {
	ownership, err := engine.InspectOwnership(mapping)
	if err != nil {
		return err
	}
	if ownership == OwnershipAbsent {
		if !force {
			return fmt.Errorf("recorded target %s is absent; use --force-targets to recreate it", mapping.Target)
		}
		return nil
	}
	if ownership == OwnershipOwned {
		return os.Remove(mapping.Target)
	}
	if force {
		return removeTargetPath(mapping.Target)
	}
	return fmt.Errorf("recorded target %s is no longer owned by source %s", mapping.Target, mapping.Source)
}

// removeRenameDestination removes a conflicting derived destination when forced.
func removeRenameDestination(engine Engine, mapping Mapping, force bool) error {
	ownership, err := engine.InspectOwnership(mapping)
	if err != nil {
		return err
	}
	if ownership == OwnershipAbsent {
		return nil
	}
	if ownership == OwnershipOwned {
		return os.Remove(mapping.Target)
	}
	if force {
		return removeTargetPath(mapping.Target)
	}
	return fmt.Errorf("rename destination target %s is no longer owned", mapping.Target)
}
