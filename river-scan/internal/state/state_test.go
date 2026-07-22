package state

import (
	"path/filepath"
	"testing"
)

func TestForgetPrefix(t *testing.T) {
	s := &State{
		Directories: map[string]DirectoryRecord{
			"/tv/Show":             {ContentHash: "a"},
			"/tv/Show/Season 1":    {ContentHash: "b"},
			"/tv/Show/Season 2":    {ContentHash: "c"},
			"/tv/Show 2":           {ContentHash: "d"}, // sibling — must NOT be cleared
			"/tv/Show 2/Season 1":  {ContentHash: "e"}, // sibling subtree — must NOT be cleared
			"/tv/Other/Season 1":   {ContentHash: "f"},
		},
		Shows: map[string]string{},
	}

	s.ForgetPrefix("/tv/Show")

	must := func(key string, present bool) {
		t.Helper()
		_, ok := s.Directories[key]
		if ok != present {
			t.Errorf("after ForgetPrefix(\"/tv/Show\"): %q present=%v, want %v", key, ok, present)
		}
	}
	must("/tv/Show",            false)
	must("/tv/Show/Season 1",   false)
	must("/tv/Show/Season 2",   false)
	must("/tv/Show 2",          true)
	must("/tv/Show 2/Season 1", true)
	must("/tv/Other/Season 1",  true)
}

// Trailing separators on the input shouldn't leave dangling subtree
// entries: "/tv/Show/" should behave identically to "/tv/Show".
func TestForgetPrefix_TrailingSeparator(t *testing.T) {
	sep := string(filepath.Separator)
	s := &State{
		Directories: map[string]DirectoryRecord{
			"/tv/Show":           {ContentHash: "a"},
			"/tv/Show/Season 1":  {ContentHash: "b"},
		},
		Shows: map[string]string{},
	}
	s.ForgetPrefix("/tv/Show" + sep)
	if _, ok := s.Directories["/tv/Show"]; ok {
		t.Errorf("/tv/Show should have been forgotten")
	}
	if _, ok := s.Directories["/tv/Show/Season 1"]; ok {
		t.Errorf("/tv/Show/Season 1 should have been forgotten")
	}
}

// Empty prefix is a no-op (defensive — wouldn't want a bad request to
// wipe the entire state).
func TestForgetPrefix_Empty(t *testing.T) {
	s := &State{
		Directories: map[string]DirectoryRecord{"/x": {ContentHash: "a"}},
		Shows:       map[string]string{},
	}
	s.ForgetPrefix("")
	if _, ok := s.Directories["/x"]; !ok {
		t.Errorf("empty prefix must not clear anything")
	}
}
