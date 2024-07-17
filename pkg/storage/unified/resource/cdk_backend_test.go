package resource

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCDK(t *testing.T) {
	backend := &cdkBackend{
		root: "some/path/in/bucket",
	}
	out := &ResourceKey{}
	key := &ResourceKey{
		Group:     "g",
		Resource:  "r",
		Namespace: "ns",
		Name:      "name",
	}
	p := backend.getPath(key, 1234)
	require.Equal(t, "some/path/in/bucket/g/r/ns/name/1234.json", p)
	backend.keyFromPath(p, out)

	require.Equal(t, key.Group, out.Group)
	require.Equal(t, key.Resource, out.Resource)
	require.Equal(t, key.Namespace, out.Namespace)
	require.Equal(t, key.Name, out.Name)
}
