// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package downloader_test

import (
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"time"

	gitjujutesting "github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	"github.com/juju/utils"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/downloader"
	"github.com/juju/juju/testing"
)

type DownloaderSuite struct {
	testing.BaseSuite
	gitjujutesting.HTTPSuite
}

func (s *DownloaderSuite) SetUpSuite(c *gc.C) {
	s.BaseSuite.SetUpSuite(c)
	s.HTTPSuite.SetUpSuite(c)
}

func (s *DownloaderSuite) TearDownSuite(c *gc.C) {
	s.HTTPSuite.TearDownSuite(c)
	s.BaseSuite.TearDownSuite(c)
}

func (s *DownloaderSuite) SetUpTest(c *gc.C) {
	s.BaseSuite.SetUpTest(c)
	s.HTTPSuite.SetUpTest(c)
}

func (s *DownloaderSuite) TearDownTest(c *gc.C) {
	s.HTTPSuite.TearDownTest(c)
	s.BaseSuite.TearDownTest(c)
}

var _ = gc.Suite(&DownloaderSuite{})

func (s *DownloaderSuite) URL(c *gc.C, path string) *url.URL {
	urlStr := s.HTTPSuite.URL(path)
	URL, err := url.Parse(urlStr)
	c.Assert(err, jc.ErrorIsNil)
	return URL
}

func (s *DownloaderSuite) testDownload(c *gc.C, hostnameVerification utils.SSLHostnameVerification) {
	tmp := c.MkDir()
	gitjujutesting.Server.Response(200, nil, []byte("archive"))
	d := downloader.StartDownload(
		downloader.Request{
			URL:       s.URL(c, "/archive.tgz"),
			TargetDir: tmp,
		},
		downloader.NewHTTPBlobOpener(hostnameVerification),
	)
	status := <-d.Done()
	c.Assert(status.Err, gc.IsNil)
	c.Assert(status.File, gc.NotNil)
	defer os.Remove(status.File.Name())
	defer status.File.Close()

	dir, _ := filepath.Split(status.File.Name())
	c.Assert(filepath.Clean(dir), gc.Equals, tmp)
	assertFileContents(c, status.File, "archive")
}

func (s *DownloaderSuite) TestDownloadWithoutDisablingSSLHostnameVerification(c *gc.C) {
	s.testDownload(c, utils.VerifySSLHostnames)
}

func (s *DownloaderSuite) TestDownloadWithDisablingSSLHostnameVerification(c *gc.C) {
	s.testDownload(c, utils.NoVerifySSLHostnames)
}

func (s *DownloaderSuite) TestDownloadError(c *gc.C) {
	gitjujutesting.Server.Response(404, nil, nil)
	d := downloader.StartDownload(
		downloader.Request{
			URL:       s.URL(c, "/archive.tgz"),
			TargetDir: c.MkDir(),
		},
		downloader.NewHTTPBlobOpener(utils.VerifySSLHostnames),
	)
	status := <-d.Done()
	c.Assert(status.File, gc.IsNil)
	c.Assert(status.Err, gc.ErrorMatches, `cannot download ".*": bad http response: 404 Not Found`)
}

func (s *DownloaderSuite) TestStopDownload(c *gc.C) {
	tmp := c.MkDir()
	d := downloader.StartDownload(
		downloader.Request{
			URL:       s.URL(c, "/x.tgz"),
			TargetDir: tmp,
		},
		downloader.NewHTTPBlobOpener(utils.VerifySSLHostnames),
	)
	d.Stop()
	select {
	case status := <-d.Done():
		c.Fatalf("received status %#v after stop", status)
	case <-time.After(testing.ShortWait):
	}
	infos, err := ioutil.ReadDir(tmp)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(infos, gc.HasLen, 0)
}
