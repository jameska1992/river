package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLibraryPathList(t *testing.T) {
	cases := []struct {
		name  string
		paths string
		want  []string
	}{
		{"object form", `[{"path":"/media/a","pre_transcoded":true},{"path":"/media/b"}]`, []string{"/media/a", "/media/b"}},
		{"legacy string form", `["/media/a","/media/b"]`, []string{"/media/a", "/media/b"}},
		{"mixed", `["/media/a",{"path":"/media/b"}]`, []string{"/media/a", "/media/b"}},
		{"empty entries skipped", `["",{"path":""},"/media/a"]`, []string{"/media/a"}},
		{"empty list", `[]`, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := (&Library{Paths: tc.paths}).PathList()
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}

	_, err := (&Library{Paths: `not json`}).PathList()
	assert.Error(t, err)
}
