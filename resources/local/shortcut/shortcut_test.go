package shortcut

import (
	"strings"
	"testing"
)

func TestShortcutAppIDHasTheTopBitSet(t *testing.T) {
	// Steam derives a shortcut's id from the executable and name, with the top
	// bit set so it can never collide with a real app id.
	id := shortcutAppID("/usr/bin/lutris", "Lutris")
	if !strings.HasPrefix(id, "-") {
		t.Errorf("appid = %q, want the high bit set (a negative int32)", id)
	}
	if shortcutAppID("/usr/bin/lutris", "Lutris") != id {
		t.Error("the same shortcut must always get the same id")
	}
	if shortcutAppID("/usr/bin/other", "Lutris") == id {
		t.Error("a different executable must get a different id")
	}
}
