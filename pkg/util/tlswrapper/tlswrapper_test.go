package tlswrapper

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	cp "github.com/otiai10/copy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReload(t *testing.T) {
	testdir := t.TempDir()
	testdir1 := filepath.Join(testdir, "set1")
	testdir2 := filepath.Join(testdir, "set2")
	testdirI := filepath.Join(testdir, "setI")
	testdirR := filepath.Join(testdir, "setR")
	require.NoError(t, cp.Copy("./testdata/set1", testdir1), "should copy dir")
	require.NoError(t, cp.Copy("./testdata/set2", testdir2), "should copy dir")
	require.NoError(t, os.Symlink(testdir1, testdirI), "should create symlink")
	require.NoError(t, os.Symlink(testdirI, testdirR), "should create symlink")

	twR, err := New(
		WithCA(filepath.Join(testdirR, "ca.crt")),
		WithLeafCert(filepath.Join(testdirR, "profile.key"), filepath.Join(testdirR, "profile.crt")),
	)
	require.NoError(t, err, "should load certificates")
	tw1, err := New(
		WithCA("./testdata/set1/ca.crt"),
		WithLeafCert("./testdata/set1/profile.key", "./testdata/set1/profile.crt"),
	)
	require.NoError(t, err, "should load certificates in set 1")
	tw2, err := New(
		WithCA("./testdata/set2/ca.crt"),
		WithLeafCert("./testdata/set2/profile.key", "./testdata/set2/profile.crt"),
	)
	require.NoError(t, err, "should load certificates in set 2")

	helloworld := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(nil) })
	newsc := func(tw TLSWrapper) (s *httptest.Server, c *http.Client) {
		s = httptest.NewUnstartedServer(helloworld)
		t.Cleanup(s.Close)
		s.Listener = tw.WrapListener(s.Listener)
		s.StartTLS()

		c = &http.Client{
			Transport: &http.Transport{
				DialTLSContext: tw.WrapDialer(&net.Dialer{Timeout: time.Second}).DialContext,
			},
		}
		return
	}
	tserverR, tclientR := newsc(twR)
	tserver1, tclient1 := newsc(tw1)
	tserver2, tclient2 := newsc(tw2)

	_, err = tclientR.Get(tserverR.URL)
	assert.NoError(t, err, "should successfully sent https request")
	_, err = tclientR.Get(tserver1.URL)
	assert.NoError(t, err, "should successfully sent https request")
	_, err = tclient1.Get(tserverR.URL)
	assert.NoError(t, err, "should successfully sent https request")
	_, err = tclientR.Get(tserver2.URL)
	assert.Error(t, err, "should fail sending https request")
	_, err = tclient2.Get(tserverR.URL)
	assert.Error(t, err, "should fail sending https request")

	require.NoError(t, os.Remove(testdirI), "should remove symlink")
	require.NoError(t, os.Symlink(testdir2, testdirI), "should create new symlink")
	fmt.Println("before remove")
	require.NoError(t, os.RemoveAll(testdir1+"/ca.crt"), "should remove old dir")
	require.NoError(t, os.RemoveAll(testdir1+"/profile.crt"), "should remove old dir")
	require.NoError(t, os.RemoveAll(testdir1+"/profile.key"), "should remove old dir")
	<-time.After(time.Second)

	tclientR.CloseIdleConnections()
	_, err = tclientR.Get(tserverR.URL)
	assert.NoError(t, err, "should successfully sent https request")
	_, err = tclientR.Get(tserver2.URL)
	assert.NoError(t, err, "should successfully sent https request")
	_, err = tclient2.Get(tserverR.URL)
	assert.NoError(t, err, "should successfully sent https request")
	_, err = tclientR.Get(tserver1.URL)
	assert.Error(t, err, "should fail sending https request")
	_, err = tclient1.Get(tserverR.URL)
	assert.Error(t, err, "should fail sending https request")
}
