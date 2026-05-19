package pathvars

import "testing"

func TestExpandSupportsTildeAndBraceVariables(t *testing.T) {
	got, err := Expand("~/config/${HOME}", Options{
		HomeDir: "/home/xenon",
		Env: map[string]string{
			"HOME": "/home/xenon",
		},
		OS: "linux",
	})
	if err != nil {
		t.Fatalf("Expand returned error: %v", err)
	}
	if got != "/home/xenon/config//home/xenon" {
		t.Fatalf("unexpected expanded path: %q", got)
	}
}

func TestExpandSupportsWindowsPercentVariables(t *testing.T) {
	got, err := Expand(`%APPDATA%\OpenCode\config.json`, Options{
		Env: map[string]string{
			"APPDATA": `C:\Users\xenon\AppData\Roaming`,
		},
		OS: "windows",
	})
	if err != nil {
		t.Fatalf("Expand returned error: %v", err)
	}
	if got != `C:\Users\xenon\AppData\Roaming\OpenCode\config.json` {
		t.Fatalf("unexpected expanded path: %q", got)
	}
}

func TestExpandRejectsUnresolvedVariables(t *testing.T) {
	_, err := Expand("${UNKNOWN}/config.json", Options{Env: map[string]string{}, OS: "linux"})
	if err == nil {
		t.Fatalf("expected unresolved variable error")
	}
}
