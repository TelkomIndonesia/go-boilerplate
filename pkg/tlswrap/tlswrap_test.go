package tlswrap

import (
	"context"
	"crypto/tls"
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
	"github.com/telkomindonesia/go-boilerplate/pkg/log"
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

	cfg := &tls.Config{ClientAuth: tls.RequireAndVerifyClientCert}
	eventR := make(chan struct{}, 10)
	twR, err := New(
		WithTLSConfig(cfg),
		WithClientCA(filepath.Join(testdirR, "ca.crt")),
		WithRootCA(filepath.Join(testdirR, "ca.crt")),
		WithLeafCert(filepath.Join(testdirR, "profile.key"), filepath.Join(testdirR, "profile.crt")),
		WithConfigReloadListener(func(s, c *tls.Config) { t.Log("event"); eventR <- struct{}{} }),
		WithLogger(log.Global().WithCtx(log.Any("test", "test"))),
	)
	require.NoError(t, err, "should load certificates")
	t.Cleanup(func() { twR.Close(context.Background()) })
	tw1, err := New(
		WithTLSConfig(cfg),
		WithClientCA("./testdata/set1/ca.crt"),
		WithRootCA("./testdata/set1/ca.crt"),
		WithLeafCert("./testdata/set1/profile.key", "./testdata/set1/profile.crt"),
	)
	require.NoError(t, err, "should load certificates in set 1")
	t.Cleanup(func() { tw1.Close(context.Background()) })
	tw2, err := New(
		WithTLSConfig(cfg),
		WithClientCA("./testdata/set2/ca.crt"),
		WithRootCA("./testdata/set2/ca.crt"),
		WithLeafCert("./testdata/set2/profile.key", "./testdata/set2/profile.crt"),
	)
	require.NoError(t, err, "should load certificates in set 2")
	t.Cleanup(func() { tw2.Close(context.Background()) })

	helloworld := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(nil) })
	newsc := func(tw TLSWrap) (s *httptest.Server, c *http.Client) {
		s = httptest.NewUnstartedServer(helloworld)
		t.Cleanup(s.Close)
		s.Listener = tw.Listener(s.Listener)
		s.StartTLS()

		c = &http.Client{
			Transport: &http.Transport{
				DialTLSContext: tw.Dialer(&net.Dialer{Timeout: time.Second}).DialContext,
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
	require.NoError(t, os.RemoveAll(testdir1), "should remove old dir")
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

	close(eventR)
	count := 0
	for range eventR {
		count += 1
	}
	assert.GreaterOrEqual(t, count, 2, "should call event listener")
}
