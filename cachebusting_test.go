package cachebusting

import (
	"embed"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

//go:embed _testdata
var embeddedFiles embed.FS

func TestNewStaticFile(t *testing.T) {
	local := "/path/to/local/file.css"
	web := "/hosted/web/path/file.css"

	sf := NewStaticFile(local, web)
	if sf.LocalPath != local {
		t.Fatal("Local path not set correctly")
		return
	}
	if sf.URLPath != web {
		t.Fatal("Web path not set correctly")
		return
	}

	return
}

func TestNewConfig(t *testing.T) {
	c := NewConfig()
	if c == nil {
		t.Fatal("Config not returned")
		return
	}
	if c.HashLength != defaultHashLength {
		t.Fatal("Default hash length not set")
		return
	}

	return
}

func TestNewOnDiskConfig(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
		return
	}

	css := NewStaticFile(filepath.Join(dir, "_testdata", "static", "css", "styles.min.css"), path.Join("/", "static", "css", "styles.min.css"))
	js := NewStaticFile(filepath.Join(dir, "_testdata", "static", "js", "script.min.js"), path.Join("/", "static", "js", "script.min.js"))
	c := NewOnDiskConfig(css, js)
	if c.HashLength != defaultHashLength {
		t.Fatal("Default hash length not set")
		return
	}
	if len(c.StaticFiles) != 2 {
		t.Fatal("Static file path pairs not saved properly")
		return
	}
}

func TestEmbeddedConfig(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
		return
	}

	css := NewStaticFile(filepath.Join(dir, "_testdata", "static", "css", "styles.min.css"), path.Join("/", "static", "css", "styles.min.css"))
	js := NewStaticFile(filepath.Join(dir, "_testdata", "static", "js", "script.min.js"), path.Join("/", "static", "js", "script.min.js"))
	c := NewEmbeddedConfig(embeddedFiles, css, js)
	if c.HashLength != defaultHashLength {
		t.Fatal("Default hash length not set")
		return
	}
	if len(c.StaticFiles) != 2 {
		t.Fatal("Static file path pairs not saved properly")
		return
	}
}

func TestValidate(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
		return
	}

	//Test Start>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	//Test with an no static files provided.
	c := NewOnDiskConfig()
	err = c.validate()
	if err != ErrNoFiles {
		t.Fatal("ErrNoFiles should have occured by didn't")
		return
	}
	//Test End<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

	//Test Start>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	//Test with an empty local path.
	css := NewStaticFile(" ", path.Join("/", "static", "css", "styles.min.css"))
	c = NewOnDiskConfig(css)
	err = c.validate()
	if err != ErrEmptyPath {
		t.Fatal("ErrEmptyPath should have occured by didn't")
		return
	}
	//Test End<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

	//Test Start>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	//Test with an empty url path.
	css = NewStaticFile(filepath.Join(dir, "_testdata", "static", "css", "styles.min.css"), " ")
	c = NewOnDiskConfig(css)
	err = c.validate()
	if err != ErrEmptyPath {
		t.Fatal("ErrEmptyPath should have occured by didn't")
		return
	}
	//Test End<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

	//Test Start>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	//Check if hash length is too short.
	css = NewStaticFile(filepath.Join(dir, "_testdata", "static", "css", "styles.min.css"), path.Join("/", "static", "css", "styles.min.css"))
	c = NewOnDiskConfig(css)
	c.HashLength = 3
	err = c.validate()
	if err != ErrHashLengthToShort {
		t.Fatal("ErrHashLengthToShort should have occured by didn't")
		return
	}
	//Test End<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

	//Test Start>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	//Check if default hash length was used if hash length was 0.
	css = NewStaticFile(filepath.Join(dir, "_testdata", "static", "css", "styles.min.css"), path.Join("/", "static", "css", "styles.min.css"))
	c = NewOnDiskConfig(css)
	c.HashLength = 0
	err = c.validate()
	if err != nil {
		t.Fatal("Error occured but should not have", err)
		return
	}
	if c.HashLength != defaultHashLength {
		t.Fatal("Default hash length should have been set but wasn't")
		return
	}
	//Test End<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

	//Test Start>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	//Make sure url paths always use forward slashes.
	css = NewStaticFile(filepath.Join(dir, "_testdata", "static", "css", "styles.min.css"), path.Join("/", "static", "css", "styles.min.css"))
	c = NewOnDiskConfig(css)
	c.HashLength = 0
	err = c.validate()
	if err != nil {
		t.Fatal("Error occured but should not have", err)
		return
	}
	for _, s := range c.StaticFiles {
		if strings.Contains(s.URLPath, "\\") {
			t.Fatal("URLPath not using forward slashes as expected", s.URLPath)
			return
		}
	}
	//Test End<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

	//Test Start>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	//If user is using embedded file, make sure embedded files were provided.
	css = NewStaticFile(filepath.Join("_testdata", "static", "css", "styles.min.css"), path.Join("/", "static", "css", "styles.min.css"))
	c = NewEmbeddedConfig(embed.FS{}, css)
	c.HashLength = 0
	err = c.validate()
	if err != ErrNoEmbeddedFilesProvided {
		t.Fatal("ErrNoEmbeddedFilesProvided should have occured but didn't")
		return
	}
	if !c.UseEmbedded {
		t.Fatal("UseEmbedded not set properly")
		return
	}
	//Test End<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

	//Test Start>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	//If user is using embedded file, make sure paths use forward slashes regardless of what user provided.
	css = NewStaticFile(filepath.Join("_testdata", "static", "css", "styles.min.css"), path.Join("/", "static", "css", "styles.min.css"))
	c = NewEmbeddedConfig(embeddedFiles, css)
	c.HashLength = 0
	err = c.validate()
	if err != nil {
		t.Fatal("Error occured should not have")
		return
	}
	for _, s := range c.StaticFiles {
		if strings.Contains(s.LocalPath, "\\") {
			t.Fatal("LocalPath not using forward slashes for embedded files as expected")
			return
		}
	}
	//Test End<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<
}

func TestCreate(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
		return
	}

	//Test Start>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	//Test validation with bad file path.
	css := NewStaticFile(" ", path.Join("/", "static", "css", "styles.min.css"))
	c := NewOnDiskConfig(css)
	err = c.Create()
	if err != ErrEmptyPath {
		t.Fatal("ErrEmptyPath should have occured by didn't")
		return
	}
	//Test End<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

	//Test Start>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	//Check if development is set and cache busting is ignored.
	css = NewStaticFile(filepath.Join(dir, "_testdata", "static", "css", "styles.min.css"), path.Join("/", "static", "css", "styles.min.css"))
	c = NewOnDiskConfig(css)
	c.Development = true
	err = c.Create()
	if err != ErrNoCacheBustingInDevelopment {
		t.Fatal("ErrNoCacheBustingInDevelopment should have occured by didn't")
		return
	}
	//Test End<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

	//Test Start>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	//Create cache busting files stored on disk and make sure new paths were saved.
	css = NewStaticFile(filepath.Join(dir, "_testdata", "static", "css", "styles.min.css"), path.Join("/", "static", "css", "styles.min.css"))
	c = NewOnDiskConfig(css)
	err = c.Create()
	if err != nil {
		t.Fatal("Error occured but should not have", err)
		return
	}
	for _, s := range c.StaticFiles {
		if s.cacheBustLocalPath == "" || !strings.Contains(s.cacheBustLocalPath, filepath.Dir(s.LocalPath)) {
			t.Fatal("Cache busting file local path not set correctly", s.cacheBustLocalPath, s.LocalPath)
			return
		}
		if s.cacheBustURLPath == "" || !strings.Contains(s.cacheBustURLPath, path.Dir(s.URLPath)) {
			t.Fatal("Cache busting url path not set correctly", s.cacheBustURLPath, s.URLPath)
			return
		}
		if len(s.fileData) > 0 {
			t.Fatal("File data should not exist but does")
			return
		}

		err = removeOldCacheBustingFiles(filepath.Dir(s.LocalPath), filepath.Base(s.LocalPath), c.HashLength)
		if err != nil {
			t.Fatal("Error cleaning up test cache busting file", s.cacheBustLocalPath, err)
			return
		}
	}
	//Test End<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

	//Test Start>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	//Create cache busting files stored in memory for on disk source and make sure new path and data were saved.
	css = NewStaticFile(filepath.Join(dir, "_testdata", "static", "css", "styles.min.css"), path.Join("/", "static", "css", "styles.min.css"))
	c = NewOnDiskConfig(css)
	c.UseMemory = true
	err = c.Create()
	if err != nil {
		t.Fatal("Error occured but should not have", err)
		return
	}
	for _, s := range c.StaticFiles {
		if s.cacheBustURLPath == "" || !strings.Contains(s.cacheBustURLPath, path.Dir(s.URLPath)) {
			t.Fatal("Cache busting url path not set correctly", s.cacheBustURLPath, s.URLPath)
			return
		}
		if s.fileData == nil {
			t.Fatal("File data should exist but does not")
			return
		}
	}
	//Test End<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

	//Test Start>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	//Create cache busting files for embedded files and make sure new paths and data were saved.
	css = NewStaticFile(filepath.Join("_testdata", "static", "css", "styles.min.css"), path.Join("/", "static", "css", "styles.min.css"))
	c = NewEmbeddedConfig(embeddedFiles, css)
	err = c.Create()
	if err != nil {
		t.Fatal("Error occured but should not have", err)
		return
	}
	for _, s := range c.StaticFiles {
		if s.cacheBustURLPath == "" || !strings.Contains(s.cacheBustURLPath, path.Dir(s.URLPath)) {
			t.Fatal("Cache busting url path not set correctly", s.cacheBustURLPath, s.URLPath)
			return
		}
		if s.fileData == nil {
			t.Fatal("File data should exist but does not")
			return
		}
	}
	//Test End<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<
}

func TestFindFileDataByCacheBustURLPath(t *testing.T) {
	//Test Start>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	//Embedded files can always be found.
	css := NewStaticFile(filepath.Join("_testdata", "static", "css", "styles.min.css"), path.Join("/", "static", "css", "styles.min.css"))
	c := NewEmbeddedConfig(embeddedFiles, css)
	err := c.Create()
	if err != nil {
		t.Fatal("Error occured but should not have", err)
		return
	}

	cssCacheBustingURL := c.StaticFiles[0].cacheBustURLPath

	data, err := c.FindFileDataByCacheBustURLPath(cssCacheBustingURL)
	if err != nil {
		t.Fatal("Error occured but should not have", err, css.URLPath)
		return
	}
	if data == nil {
		t.Fatal("No data was returned as expected")
		return
	}
	//Test End<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

	//Test Start>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	//Test a file that doesn't exist.
	css = NewStaticFile(filepath.Join("_testdata", "static", "css", "styles.min.css"), path.Join("/", "static", "css", "styles.min.css"))
	c = NewEmbeddedConfig(embeddedFiles, css)
	err = c.Create()
	if err != nil {
		t.Fatal("Error occured but should not have", err)
		return
	}

	_, err = c.FindFileDataByCacheBustURLPath(css.URLPath + ".old")
	if err != ErrNotFound {
		t.Fatal("ErrNotFound should have occured but didn't")
		return
	}
	//Test End<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

	//Test Start>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	//Test on disk not in memory config, nothing should be returned since file is stored on disk
	css = NewStaticFile(filepath.Join("_testdata", "static", "css", "styles.min.css"), filepath.Join("/", "static", "css", "styles.min.css"))
	c = NewOnDiskConfig(css)
	err = c.Create()
	if err != nil {
		t.Fatal("Error occured but should not have", err)
		return
	}

	_, err = c.FindFileDataByCacheBustURLPath(css.URLPath)
	if err != ErrFileNotStoredInMemory {
		t.Fatal("ErrFileNotStoredInMemory should have occured but didn't")
		return
	}
	//Test End<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<
}

func TestGetFilenamePairs(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
		return
	}

	css := NewStaticFile(filepath.Join(dir, "_testdata", "static", "css", "styles.min.css"), path.Join("/", "static", "css", "styles.min.css"))
	c := NewOnDiskConfig(css)
	err = c.Create()
	if err != nil {
		t.Fatal("Error occured but should not have", err)
		return
	}

	pairs := c.GetFilenamePairs()
	if len(pairs) != 1 {
		t.Fatal("No filename pairs returned as expected")
		return
	}
}

func TestDefaultConfig(t *testing.T) {
	//Test Start>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	//GetConfig()
	css := NewStaticFile(filepath.Join("_testdata", "static", "css", "styles.min.css"), filepath.Join("/", "static", "css", "styles.min.css"))
	DefaultOnDiskConfig(css)
	c := GetConfig()
	if c.StaticFiles[0].LocalPath != css.LocalPath {
		t.Fatal("Default config not saved correctly")
		return
	}
	//Test End<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

	//Test Start>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	//HashLength
	HashLength(23)
	c = GetConfig()
	if c.HashLength != 23 {
		t.Fatal("HashLength field not set correctly")
		return
	}
	//Test End<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

	//Test Start>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	//Development
	Development(true)
	c = GetConfig()
	if !c.Development {
		t.Fatal("Development field not set correctly")
		return
	}
	//Test End<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

	//Test Start>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	//Debug
	Debug(true)
	c = GetConfig()
	if !c.Debug {
		t.Fatal("Debug field not set correctly")
		return
	}
	//Test End<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

	//Test Start>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
	//UseMemory
	UseMemory(true)
	c = GetConfig()
	if !c.UseMemory {
		t.Fatal("UseMemory field not set correctly")
		return
	}
	//Test End<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<
}
