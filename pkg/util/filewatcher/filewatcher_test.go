package filewatcher

import (
	"context"
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFileContentWatcherSymlink for https://pkg.go.dev/k8s.io/kubernetes/pkg/volume/util#AtomicWriter
func TestFileContentWatcherSymlink(t *testing.T) {
	fname := path.Join(os.TempDir(), fmt.Sprintf("%s%d", t.Name(), time.Now().UnixNano()))
	err := os.WriteFile(fname, []byte("1"), 0644)
	require.NoError(t, err, "should create temporary file")
	defer os.Remove(fname)

	fnameS1, fnameS2 := fname+"-1", fname+"-2"
	require.NoError(t, os.Symlink(fname, fnameS1), "should create first symlink")
	defer os.Remove(fnameS1)
	require.NoError(t, os.Symlink(fnameS1, fnameS2), "should create second symlink")
	defer os.Remove(fnameS2)

	count := 0
	fw, err := New(fnameS2, func(name string, err error) {
		if err != nil {
			t.Log(err)
			return
		}
		count += 1
	})
	require.NoError(t, err, "should create file watcher")
	defer fw.Close(context.Background())

	<-time.After(time.Second)
	fnameNew := fname + "-new"
	err = os.WriteFile(fnameNew, []byte("2"), 0644)
	require.NoError(t, err, "should create new file")
	defer os.Remove(fnameNew)
	require.NoError(t, os.Remove(fnameS1), "should delete old symlink")
	require.NoError(t, os.Symlink(fnameNew, fnameS1), "should create new symlink")
	require.NoError(t, os.Remove(fname), "should delete old file")

	<-time.After(3 * time.Second)
	assert.Greater(t, count, 0, "should invoke callback")
}

func TestFileContentWatcherWrite(t *testing.T) {
	fname := path.Join(os.TempDir(), fmt.Sprintf("%s%d", t.Name(), time.Now().UnixNano()))
	err := os.WriteFile(fname, []byte("hello world"), 0644)
	require.NoError(t, err, "should create temporary file")
	defer os.Remove(fname)

	count := 0
	fw, err := New(fname, func(name string, err error) {
		if err != nil {
			t.Log(err)
			return
		}
		assert.Equal(t, fname, name, "should return changed file name")
		count += 1
	})
	require.NoError(t, err, "should create file watcher")
	defer fw.Close(context.Background())

	<-time.After(time.Second)
	err = os.WriteFile(fname, []byte("hellow world 2"), 0644)
	require.NoError(t, err, "should update temporary file")

	<-time.After(time.Second)
	assert.Greater(t, count, 0, "should invoke callback")
}
