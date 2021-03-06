package plugin

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func tempd(t *testing.T) string {
	tmpd, err := ioutil.TempDir("", "mkr-plugin-install")
	if err != nil {
		t.Fatal(err)
	}
	return tmpd
}

func assertEqualFileContent(t *testing.T, aFile, bFile, message string) {
	aContent, err := ioutil.ReadFile(aFile)
	if err != nil {
		t.Fatal(err)
	}
	bContent, err := ioutil.ReadFile(bFile)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, aContent, bContent, message)
}

func TestSetupPluginDir(t *testing.T) {
	{
		// Creating plugin dir is successful
		tmpd := tempd(t)
		defer os.RemoveAll(tmpd)

		pluginDir, err := setupPluginDir(tmpd)
		assert.Equal(t, tmpd, pluginDir, "returns default plugin directory")
		assert.Nil(t, err, "setup finished successfully")

		fi, err := os.Stat(filepath.Join(tmpd, "bin"))
		if assert.Nil(t, err) {
			assert.True(t, fi.IsDir(), "plugin bin directory is created")
		}

		fi, err = os.Stat(filepath.Join(tmpd, "work"))
		if assert.Nil(t, err) {
			assert.True(t, fi.IsDir(), "plugin work directory is created")
		}
	}

	{
		// Creating plugin dir is failed because of directory's permission
		tmpd := tempd(t)
		defer os.RemoveAll(tmpd)
		err := os.Chmod(tmpd, 0500)
		assert.Nil(t, err, "chmod finished successfully")

		pluginDir, err := setupPluginDir(tmpd)
		assert.Equal(t, "", pluginDir, "returns empty string when failed")
		assert.NotNil(t, err, "error should be occured while manipulate unpermitted directory")
	}
}

func TestDownloadPluginArtifact(t *testing.T) {
	ts := httptest.NewServer(http.FileServer(http.Dir("testdata")))
	defer ts.Close()

	{
		// Response not found
		tmpd := tempd(t)
		defer os.RemoveAll(tmpd)

		fpath, err := downloadPluginArtifact(ts.URL+"/not_found.zip", tmpd)
		assert.Equal(t, "", fpath, "fpath is empty")
		assert.Contains(t, err.Error(), "http response not OK. code: 404,", "Returns correct err")
	}

	{
		// Download is finished successfully
		tmpd := tempd(t)
		defer os.RemoveAll(tmpd)

		fpath, err := downloadPluginArtifact(ts.URL+"/mackerel-plugin-sample_linux_amd64.zip", tmpd)
		assert.Equal(t, tmpd+"/mackerel-plugin-sample_linux_amd64.zip", fpath, "Returns fpath correctly")

		_, err = os.Stat(fpath)
		assert.Nil(t, err, "Downloaded file is created")

		assertEqualFileContent(t, fpath, "testdata/mackerel-plugin-sample_linux_amd64.zip", "Downloaded data is correct")
	}
}

func TestInstallByArtifact(t *testing.T) {
	{
		// Install by the artifact which has a single plugin
		bindir := tempd(t)
		defer os.RemoveAll(bindir)
		workdir := tempd(t)
		defer os.RemoveAll(workdir)

		err := installByArtifact("testdata/mackerel-plugin-sample_linux_amd64.zip", bindir, workdir, false)
		assert.Nil(t, err, "installByArtifact finished successfully")

		installedPath := filepath.Join(bindir, "mackerel-plugin-sample")

		fi, err := os.Stat(installedPath)
		assert.Nil(t, err, "A plugin file exists")
		assert.True(t, fi.Mode().IsRegular() && fi.Mode().Perm() == 0755, "A plugin file has execution permission")
		assertEqualFileContent(
			t,
			installedPath,
			"testdata/mackerel-plugin-sample_linux_amd64/mackerel-plugin-sample",
			"Installed plugin is valid",
		)

		// Install same name plugin, but it is skipped
		workdir2 := tempd(t)
		defer os.RemoveAll(workdir2)
		err = installByArtifact("testdata/mackerel-plugin-sample-duplicate_linux_amd64.zip", bindir, workdir2, false)
		assert.Nil(t, err, "installByArtifact finished successfully even if same name plugin exists")

		fi, err = os.Stat(filepath.Join(bindir, "mackerel-plugin-sample"))
		assert.Nil(t, err, "A plugin file exists")
		assertEqualFileContent(
			t,
			installedPath,
			"testdata/mackerel-plugin-sample_linux_amd64/mackerel-plugin-sample",
			"Install is skipped, so the contents is what is before",
		)

		// Install same name plugin with overwrite option
		workdir3 := tempd(t)
		defer os.RemoveAll(workdir3)
		err = installByArtifact("testdata/mackerel-plugin-sample-duplicate_linux_amd64.zip", bindir, workdir3, true)
		assert.Nil(t, err, "installByArtifact finished successfully")
		assertEqualFileContent(
			t,
			installedPath,
			"testdata/mackerel-plugin-sample-duplicate_linux_amd64/mackerel-plugin-sample",
			"a plugin is installed with overwrite option, so the contents is overwritten",
		)
	}

	{
		// Install by the artifact which has multiple plugins
		bindir := tempd(t)
		defer os.RemoveAll(bindir)
		workdir := tempd(t)
		defer os.RemoveAll(workdir)

		installByArtifact("testdata/mackerel-plugin-sample-multi_darwin_386.zip", bindir, workdir, false)

		// check-sample, mackerel-plugin-sample-multi-1 and plugins/mackerel-plugin-sample-multi-2
		// are installed.  But followings are not installed
		// - mackerel-plugin-non-executable: does not have execution permission
		// - not-mackerel-plugin-sample: does not has plugin file name
		assertEqualFileContent(t,
			filepath.Join(bindir, "check-sample"),
			"testdata/mackerel-plugin-sample-multi_darwin_386/check-sample",
			"check-sample is installed",
		)
		assertEqualFileContent(t,
			filepath.Join(bindir, "mackerel-plugin-sample-multi-1"),
			"testdata/mackerel-plugin-sample-multi_darwin_386/mackerel-plugin-sample-multi-1",
			"mackerel-plugin-sample-multi-1 is installed",
		)
		assertEqualFileContent(t,
			filepath.Join(bindir, "mackerel-plugin-sample-multi-2"),
			"testdata/mackerel-plugin-sample-multi_darwin_386/plugins/mackerel-plugin-sample-multi-2",
			"mackerel-plugin-sample-multi-2 is installed",
		)

		_, err := os.Stat(filepath.Join(bindir, "mackerel-plugin-not-executable"))
		assert.NotNil(t, err, "mackerel-plugin-not-executable is not installed")
		_, err = os.Stat(filepath.Join(bindir, "not-mackerel-plugin-sample"))
		assert.NotNil(t, err, "not-mackerel-plugin-sample is not installed")
	}
}

func TestLooksLikePlugin(t *testing.T) {
	testCases := []struct {
		Name            string
		LooksLikePlugin bool
	}{
		{"mackerel-plugin-sample", true},
		{"mackerel-plugin-hoge_sample1", true},
		{"check-sample", true},
		{"check-hoge-sample", true},
		{"mackerel-sample", false},
		{"hoge-mackerel-plugin-sample", false},
		{"hoge-check-sample", false},
		{"wrong-sample", false},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.LooksLikePlugin, looksLikePlugin(tc.Name))
	}
}
