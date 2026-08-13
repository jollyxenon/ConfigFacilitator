package content

import "errors"

// makeFIFO reports that named-pipe test setup is unavailable on native Windows.
func makeFIFO(path string) error {
	return errors.New("named-pipe test setup is unavailable on Windows")
}
