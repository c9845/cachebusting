/*
Package cachebusting handles creation of static files that will not be found in browser
caches to prevent serving of old version of a file causing webpage/webapp performance or
execution issues.

Cache busting is done at runtime by calculating a hash of each defined, original file (for
example, script.min.js), creating a new name for the file by appending the hash to the
beginning of the original file name, and saving a copy of the original file to disk or to
the app's memory. This package then provides a key-value matching of the original file name
to the cache busting file name that can be used to handle replacement of file names in
<link> or <script> tags.

This supports embedded files (using the go embed package). To save a copy of the embedded
file for serving under the cache busting file name, since the app cannot write to the
embedded filesystem, the app saves the copy to memory. This can also be used when the original
file is stored on disk in cases where your app cannot write to disk.

To use the cache busted version of each file, modify your html templates to replace usage
of an original file with the cache busted version by matching up the original name of the
minified file.
For example:
<html>
  <head>
    {{$originalFile := "styles.min.css"}}
	{{$cacheBustFiles := .CacheBustFiles}}

	{{/*If the key "styles.min.css" exists in $cacheBustFiles, then the associated cache-busted filename will be returned as {{.}}. *\/}}
	{{with index $cacheBustFiles $originalFile}}
	  {{$cacheBustedFile := .}}
	  <link rel="stylesheet" href="/static/css/{{$cacheBustedFile}}">
    {{else}}
      <link rel="stylesheet" href="/static/css/{{$originalFile}}">
    {{end}}
  </head>
</html>

The expected local directory format for your static files is as follows:
website/
├─ static/
│  ├─ css
│  │  ├─ styles.min.css
│  ├─ js
│  │  ├─ script.min.js

The expected paths for each file as served from a browser is noted as follows:
- example.com/static/css/{hash-prefix}.styles.min.css
- example.com/static/js/{hash-prefix}.script.min.jss
*/
package cachebusting

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
)

//StaticFile contains the local path to the on disk or embedded original static file
//and the URL path on which the file is served. We use the local path to look up the
//file and create the cache busting version of the file. We use the URL the file is
//served on to reply with the correct file's contents if the file is stored in the
//app's memory (for embedded file or if UseMemory is true).
type StaticFile struct {
	//Local path is the full, complete path to the original copy of the static file
	//you want to cache bust. This is the path to the file on disk or emebedded in
	//the app using the embed package.
	//
	//Caution of using the correct slash style. It is best to construct the path using
	//the 'filepath' package (or the 'path' package for embedded files).
	//
	//If you are using embedded files, each path must start at the root folder you are
	//embedding. I.e.: if you use //go: embed website/static/js/script.min.js then the
	//path here must be "/website/static/js/script.min.js". You must have also read the
	//embedded files, with code such as var embeddedFiles embed.FS, prior and you must
	//provide the embed.FS to the CacheBustConfig.
	//
	//Ex.: /path/to/static/js/script.min.js
	LocalPath string

	//URLPath is the path off your domain, not including the domain, at which you
	//would serve the original static file. This is used to construct the path to
	//the cache busting file on which the cache busting file will be served so that
	//you can match up requests for the cache busting file with the file's data. This
	//is only used when storing the cache busting file in memory since if the cache
	//busting file is saved to disk, it will be saved in the same directory as the
	//original file and thus you can simply serve the static directory using an http
	//route and os.DirFS and http.FileServer.
	//
	//Always use forward slashes! It is best to construct the path using the 'path'
	//package.
	//
	//Ex.: /static/js/script.min.js
	URLPath string

	//cacheBustLocalPath is the full, complete path to the cache busting copy of the
	//file. This is constructed from the LocalPath and the cache busting file's name
	//if the cache busting files are not stored in memory.
	cacheBustLocalPath string

	//cacheBustURLPath is the path of your domain at which the cache busting file
	//will serve. This is constructed from the URLPath and the cache busting file's
	//name. This is only used when serving files from memory since if you are serving
	//files from disk it is easier to just serve the directory the files are located
	//in using os.DirFS and http.FileServer (see http handler below).
	cacheBustURLPath string

	//fileData stores the contents of the cache busting file when the cache busting
	//file is stored in memory (for embedded files or if UseMemory is true). This is
	//simply a copy of the file at the time creation of the cache busting file is
	//performed. This is the file's data when it is stored in memory.
	fileData []byte
}

//Config is the set of configuration settings for cache busting.
type Config struct {
	//Development is used to disable cache busting.
	Development bool

	//Debug enables printing out diagnostic information.
	Debug bool

	//HashLength defines the number of characters prepended to each original file's name
	//to create the cache busting file's name.
	HashLength uint

	//StaticFiles is the list of files to cache bust.
	StaticFiles []StaticFile

	//UseEmbedded means files built into the golang executable will be used rather than
	//files stored on-disk. You must have read the embedded files, with code such as
	//var embeddedFiles embed.FS, prior and you must provide the embed.FS to the EmbeddedFS.
	UseEmbedded bool

	//EmbeddedFiles is the filesystem embedded into this executable via the embed package.
	//You must have read the embedded files, with code such as var embeddedFiles embed.FS,
	//prior and you must set UseEmbedded to true to enable use of these files.
	EmbeddedFS embed.FS

	//UseMemory causes the cache busting copy of each file to be stored in the app's
	//memory versus on disk. This is only applicable when you are using original files
	//stored on disk since if you are using embedded files the copies will always be
	//stored in memory. This is useful for times when your app is running on a system
	//that cannot write to disk.
	UseMemory bool
}

//default values
const (
	//minHashLength is just a value chosen for the shortest hash length we want to support.
	minHashLength = uint(8)

	//defaultHashLength is the hash length we will use unless the user provides a value in
	//their config's HashLength field that is longer than minHashLength.
	defaultHashLength = minHashLength
)

//errors
var (
	//ErrNoFiles is returned when no static files were provided to cache bust.
	ErrNoFiles = errors.New("cachebusting: no files provided")

	//ErrEmptyPath is returned when a static file's local or url path is blank.
	ErrEmptyPath = errors.New("cachebusting: empty path provided is invalid")

	//ErrNoEmbeddedFilesProvided is returned when a user is using a config with embedded files
	//but no embedded files were provided.
	ErrNoEmbeddedFilesProvided = errors.New("cachebusting: no embedded files provided")

	//ErrNoCacheBustingInDevelopment is returned when CreateCacheBustingFiles() is called
	//but the config's Development field is set to True.
	ErrNoCacheBustingInDevelopment = errors.New("cachebusting: disabled because Development field is true")

	//ErrHashLengthToShort is returned when a too short hash length is provided to the config.
	ErrHashLengthToShort = errors.New("cachebusting: hash length too short, must be at least " + strconv.FormatUint(uint64(minHashLength), 10))

	//ErrFileNotStoredInMemory is returned when a user tries to look up a file's data but
	//that file's data is stored on disk, not in memory.
	ErrFileNotStoredInMemory = errors.New("cachebusting: file not stored in memory")

	//ErrNotFound is returned when a user tries to look up a file in the list of static files
	//but the file data cannot be found. This means the file was not cache-busted.
	ErrNotFound = errors.New("cachebusting: file not found")
)

//config is the package level saved config. This stores your config when you want to store
//it for global use. It is populated when you use one of the Default...Config() funcs.
var config Config

//NewStaticFile returns an object for a static file with the paths defined. This is just a
//helper func around creating the StaticFile object.
func NewStaticFile(localPath, urlPath string) StaticFile {
	return StaticFile{
		LocalPath: localPath,
		URLPath:   urlPath,
	}
}

//NewConfig returns a config for managing your cache bust files with some defaults set.
func NewConfig() *Config {
	return &Config{
		HashLength: defaultHashLength,
	}
}

//DefaultConfig initializes the package level config with some defaults set. This wraps
//NewConfig() and saves the config to the package.
func DefaultConfig() {
	cfg := NewConfig()
	config = *cfg
}

//NewOnDiskConfig returns a config for managing your cache busted files when the original
//files are stored on disk.
func NewOnDiskConfig(files ...StaticFile) *Config {
	return &Config{
		HashLength:  defaultHashLength,
		StaticFiles: files,
	}
}

//DefaultOnDiskConfig initializes the package level config with the provided static files
//and some defaults.
func DefaultOnDiskConfig(files ...StaticFile) {
	cfg := NewOnDiskConfig(files...)
	config = *cfg
}

//NewEmbeddedConfig returns a config for managing your cache busted files when the original
//files embedded in the app.
func NewEmbeddedConfig(e embed.FS, files ...StaticFile) *Config {
	return &Config{
		HashLength:  defaultHashLength,
		StaticFiles: files,
		UseEmbedded: true,
		EmbeddedFS:  e,
	}
}

//DefaultEmbeddedConfig initializes the package level config with the provided static files
//and some defaults.
func DefaultEmbeddedConfig(e embed.FS, files ...StaticFile) {
	cfg := NewEmbeddedConfig(e, files...)
	config = *cfg
}

//validate handles validation of a provided config.
func (c *Config) validate() (err error) {
	//check if no files were provided.
	if len(c.StaticFiles) == 0 {
		return ErrNoFiles
	}

	for k, s := range c.StaticFiles {
		//check if any file paths are blank.
		l := strings.TrimSpace(s.LocalPath)
		u := strings.TrimSpace(s.URLPath)
		if l == "" || u == "" {
			return ErrEmptyPath
		}

		//make sure if user is using embedded file, the paths use a "/" separator.
		if c.UseEmbedded {
			l = filepath.ToSlash(l)
			c.StaticFiles[k].LocalPath = l
		}

		//make sure url paths use a "/" separator and path starts with a "/".
		//Join adds the "/" in case the user forgot it, Clean removes any double "//"
		//in cases where user did add "/" and we just added another.
		u = path.Clean(path.Join("/", filepath.ToSlash(u)))
		c.StaticFiles[k].URLPath = u
	}

	//check if the static hash length was provided or is too short
	if c.HashLength == 0 {
		c.HashLength = defaultHashLength
	} else if c.HashLength < minHashLength {
		return ErrHashLengthToShort
	}

	//if user is using embedded files, make sure something was provided.
	if c.UseEmbedded && c.EmbeddedFS == (embed.FS{}) {
		return ErrNoEmbeddedFilesProvided
	}

	return
}

//Create handles the creation of the cache busting files and associated data. This calculates
//a hash of each static file, creates a copy of the static file, and saves the copy referenced
//by a new name using the hash. The copy of the original static file is either saved to disk
//(for original files stored on disk) or in memory (for embedded files or if the config's
//UseMemory field is set to true). This also saves some info for use in serving each cache
//busting copy of the static original file.
func (c *Config) Create() (err error) {
	//validate the config
	err = c.validate()
	if err != nil {
		return
	}

	//ignore creating cache busting files in development.
	if c.Development {
		if c.Debug {
			log.Println("cachebusting.Create (debug)", "creation of cache busting files is disabled, config field Development is true")
		}

		return ErrNoCacheBustingInDevelopment
	}

	//determine the correct func to use for reading original file's data.
	//We aren't using Open(), even though that would have been nicer, since os.Open (for on
	//disk files) returns a *File type while embed.Open (for embedded files) returns just a
	//File type (notice no pointer *).
	var readFunc func(string) ([]byte, error)
	if c.UseEmbedded {
		readFunc = c.EmbeddedFS.ReadFile
	} else {
		readFunc = os.ReadFile
	}

	//Handle each static file.
	//This will:
	// 1) Hash the file to create a somewhat random and unique element to prepend to the file's name.
	// 2) Create a copy of the file, either on disk or in memory, using the hash and original file's name.
	// 3) Store some info about each cache busting file.
	for k, s := range c.StaticFiles {
		//use correct path separator
		//If using embedded files, the path separator is always "/" so we need to parse
		//the path as such in case user used filepath.Join to build the path and thus the
		//file's local path has possibly Windows "\" separators.
		originalPath := s.LocalPath
		if c.UseEmbedded {
			originalPath = filepath.ToSlash(s.LocalPath)
		}

		//get just the name of the static file
		//This is used as a base to create the filename of the cache busting file. The
		//hash calculated from the file's data is prepended to this.
		originalFilename := filepath.Base(originalPath)

		//get just the directory of the static file
		//This is used for removing old cache busting files from this directory as well
		//as saving the new cache busting file
		originalDirectory := filepath.Dir(s.LocalPath)

		//remove any old cache busting files if the files are stored on disk.
		//This prevents the filesystem from getting clogged up with all sorts of old
		//unneeded files.
		if !c.UseEmbedded && !c.UseMemory {
			innerErr := removeOldCacheBustingFiles(originalDirectory, originalFilename, c.HashLength)
			if err != nil {
				return innerErr
			}
		}

		//read in the original file
		originalFile, innerErr := readFunc(originalPath)
		if innerErr != nil {
			return innerErr
		}

		//calculate hash of the original file's data
		//This gives us a random and unique element we can prepend to the file's name
		//so that the file's name will change if the contents have changed therefore
		//not using the browser cached version of the file.
		h := sha256.Sum256(originalFile)
		hash := strings.ToUpper(hex.EncodeToString(h[:]))

		//trim the hash as needed.
		if c.HashLength == 0 {
			//double check even though this should have been caught in validate.
			//use default.
			hash = hash[:defaultHashLength]
		} else if int(c.HashLength) > len(hash) {
			//hash length set in config is longer then the actual hash.
			//use entire hash.

		} else {
			//use hash length set in config
			hash = hash[:c.HashLength]
		}

		//create the filename for the cache busting copy of the file
		cachebustFilename := hash + "." + originalFilename

		//save a copy of the file's contents
		//When saving a file back to disk, the default for original files stored on
		//disk, this simply saves a copy of the file with the new name back to the
		//same directory.
		//For embedded files, or when UseMemory is true for original files stored on
		//disk, this saves a copy of the file to the app's memory.
		if !c.UseEmbedded && !c.UseMemory {
			cachebustPath := filepath.Join(originalDirectory, cachebustFilename)

			f, innerErr := os.Create(cachebustPath)
			if innerErr != nil {
				return innerErr
			}
			defer f.Close()

			_, innerErr = f.Write(originalFile)
			if innerErr != nil {
				return innerErr
			}
			f.Close()

			if c.Debug {
				log.Println("cachebusting.Create (debug)", "copying cache busting files to", cachebustPath)
			}

			c.StaticFiles[k].cacheBustLocalPath = cachebustPath

		} else {
			c.StaticFiles[k].fileData = originalFile
			c.StaticFiles[k].cacheBustLocalPath = cachebustFilename + " (in memory)" //diagnostics
		}

		//save the url path/endpoint this file should be served on
		//This is built from the path the original static file would be served on and
		//replaces the original filename with the cache bust filename. This is used for
		//matching up endpoints which what file to serve and is really only needed when
		//you are serving files from memory since if you are serving files from disk you
		//can use os.DirFS and http.FileServer. Using path here, not filepath, since we
		//always want to treat the output as separated by "/".
		c.StaticFiles[k].cacheBustURLPath = path.Join(path.Dir(s.URLPath), cachebustFilename)
	}

	//the below code is messy, I am aware
	if c.Debug {
		//tabwriter used to organize logging output better
		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 1, ' ', tabwriter.Debug)

		log.Println("cachebusting.Create (debug)", "cache busted files matching...")
		cols := []string{"ORIGINAL FILENAME", "CACHEBUST FILENAME"}
		fmt.Fprintln(tw, strings.Join(cols, "\t"))
		for _, v := range c.StaticFiles {
			cols := []string{filepath.Base(v.LocalPath), filepath.Base(v.cacheBustLocalPath)}
			fmt.Fprintln(tw, strings.Join(cols, "\t"))
		}
		tw.Flush()

		log.Println("")

		log.Println("cachebusting.Create (debug)", "cache busted url matching...")
		cols = []string{"ORIGINAL URL PATH", "CACHEBUST URL PATH"}
		fmt.Fprintln(tw, strings.Join(cols, "\t"))
		for _, v := range c.StaticFiles {
			cols = []string{v.URLPath, v.cacheBustURLPath}
			fmt.Fprintln(tw, strings.Join(cols, "\t"))
		}
		tw.Flush()
	}

	return
}

//Create handles creation of the cache busting files using the default package level config.
func Create() (err error) {
	err = config.Create()
	return
}

//removeOldCacheBustingFiles deletes already existing cache busting files from a given
//directory. This prevents the directory from needlessly getting filled up with unused
//files.
//
//This works by looking for any files in the directory that contain the original file's name
//and has a hash prepended to it. We cannot just remove any file that has the file's name
//since that would also remove the original source file! We could mistakenly delete other
//files that (1) contain the file's name and (2) are prepended by the same amount of characters
//as the hash we use, the chances of this are slim though.
func removeOldCacheBustingFiles(directory, originalFilename string, hashLength uint) error {
	//get list of files in the directory
	files, err := os.ReadDir(directory)
	if err != nil {
		return err
	}

	//check if each file is an old cache busting file.
	for _, f := range files {
		if f.IsDir() {
			return err
		}

		//we know our hash only contains uppercase A-F and 0-9 digits since we are encoding
		//the hash to uppercase hexidecimal.
		exp := "[A-F0-9]{" + strconv.FormatUint(uint64(hashLength), 10) + "}." + originalFilename

		//we aren't using regexp.MustCompile here since the expression changes with user input,
		//the expression isn't hardcoded in the app, so we want to return the error rather then
		//just panicing.
		r, err := regexp.Compile(exp)
		if err != nil {
			return err
		}

		if r.MatchString(f.Name()) {
			pathToOldFile := filepath.Join(directory, f.Name())
			removeErr := os.Remove(pathToOldFile)
			if removeErr != nil {
				return removeErr
			}
		}
	}

	return nil
}

//FindFileDataByCacheBustURLPath returns a StaticFile's file data for the given url. This url
//is the url path the browser is requesting and should be the cache busting URL, not the
//original static file url. This is used when serving files but only when files are stored in
//memory.
func (c *Config) FindFileDataByCacheBustURLPath(urlPath string) (b []byte, err error) {
	if c.Debug {
		log.Println("cachebusting.FindFileDataByCacheBustURLPath (debug)", urlPath)
	}

	if !c.UseEmbedded && !c.UseMemory {
		err = ErrFileNotStoredInMemory
		return
	}

	for _, v := range c.StaticFiles {
		if v.cacheBustURLPath == urlPath {
			b = v.fileData
			return
		}
	}

	err = ErrNotFound
	return
}

//FindFileDataByCacheBustURLPath wraps FindFileDataByCacheBustURLPath for the package level config.
func FindFileDataByCacheBustURLPath(path string) (b []byte, err error) {
	return config.FindFileDataByCacheBustURLPath(path)
}

//GetConfig returns the current state of the package level config.
func GetConfig() *Config {
	return &config
}

//GetFilenamePairs returns the original to cache busting filename pairs.
func (c *Config) GetFilenamePairs() (pairs map[string]string) {
	pairs = make(map[string]string)

	for _, v := range c.StaticFiles {
		original := filepath.Base(v.LocalPath)
		cachebust := filepath.Base(v.cacheBustURLPath)

		pairs[original] = cachebust
	}

	return
}

//GetFilenamePairs returns the file pairs for the package level config.
func GetFilenamePairs() (pairs map[string]string) {
	return config.GetFilenamePairs()
}

//StaticFileHandler is an example func that can be used to serve static files whether you
//are using embedded or on-disk original files and in memory or on disk cache busting files.
//You would use this func in your http router. This is an example since it requires a strict
//local directory structure and strict url path to each static file.
//Notes:
// - See package level comment about expected directory structure.
// - Extra headers added for diagnosing where files are stored in browser dev tools.
// - Set cacheDays to 0 to prevent caching in the user's browser.
func (c *Config) StaticFileHandler(cacheDays int, pathToStaticFiles string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//set header to control caching of file in user's browser
		//max age is in days
		//if value is 0, files won't be cached in browser
		maxAge := cacheDays * 24 * 60 * 60
		w.Header().Set("Cache-Control", "no-transform,public,max-age="+strconv.Itoa(maxAge))

		//serve the file being requested.
		//Cache busting files will be stored in the app's memory if the app is using embedded
		//files or the app is storing cache busting versions of on disk files in memory (i.e.
		//app is deployed on a system that doesn't allow writing to disk). If the file cannot
		//be found and served, the file being requested is most likely a vendor file.
		if c.UseEmbedded || c.UseMemory {
			//try finding cache busting file in memory.
			fd, err := FindFileDataByCacheBustURLPath(r.URL.Path)
			if err == nil {
				w.Header().Set("X-Static-Served-From", "memory")
				w.Header().Set("Content-Type", mime.TypeByExtension(path.Ext(r.URL.Path)))
				w.Write(fd)
				return
			} else if err != ErrNotFound {
				log.Println("cachebusting.StaticFileHandler", "odd error serving file from memory", err)
			}
		}

		//serve files that couldn't be found in app's memory.
		//This is with a cache busting file saved to disk (default when original static is
		//stored on disk) or a vendor file. Get the correct list of filesystem based on if
		//the app is using embedded files or files stored on disk.
		var httpFS http.FileSystem
		if c.UseEmbedded {
			w.Header().Set("X-Static-Served-From", "embedded")

			//dir is equivalent to "/" now. This doesn't work for us because requests
			//are coming in for files with url paths starting at /static/.
			//Note: See package level comment about expected directory structure.
			rootDir := c.EmbeddedFS

			//change to the /website directory. Inside this directory is the static
			//directory where files are stored. The directory structure now matches the
			//request path.
			const dirName = "website"
			websiteDir, err := fs.Sub(rootDir, dirName)
			if err != nil {
				log.Println("cachebusting.StaticFileHandler", "could not find "+dirName+" in embedded files.", err)
				return
			}

			//serve the /website directory where static/... is located
			httpFS = http.FS(websiteDir)
		} else {
			w.Header().Set("X-Static-Served-From", "disk")

			//This was the old way of serving static files before support for embedded files existed.
			//os.DirFS opens the "website" directory so that when a path is requested starting with
			//"static", the directory structure will match the url path.
			dir := os.DirFS(pathToStaticFiles)
			httpFS = http.FS(dir)
		}

		fileserver := http.FileServer(httpFS)
		fileserver.ServeHTTP(w, r)
		return
	})
}

//DefaultStaticFileHandler is an example handler for serving static files using the
//package level saved config.
func DefaultStaticFileHandler(cacheDays int, pathToStaticFiles string) http.Handler {
	return config.StaticFileHandler(cacheDays, pathToStaticFiles)
}

//PrintEmbeddedFileList prints out the list of files embedded into the executable. This should
//be used for diagnostics purposes only to confirm which files are embedded with the //go:embed
//directives elsewhere in your app.
func PrintEmbeddedFileList(e embed.FS) {
	//the directory "." means the root directory of the embedded file.
	const startingDirectory = "."

	err := fs.WalkDir(e, startingDirectory, func(path string, d fs.DirEntry, err error) error {
		log.Println(path)
		return nil
	})
	if err != nil {
		log.Fatalln("cachebusting.PrintEmbeddedFiles", "error walking embedded directory", err)
		return
	}

	//exit after printing since you should never need to use this function outside of testing
	//or development.
	log.Println("cachebusting.PrintEmbeddedFiles", "os.Exit() called, remove or skip PrintEmbeddedFileList to continue execution.")
	os.Exit(0)
}

//HashLength sets the HashLength field on the package level config.
func HashLength(l uint) {
	config.HashLength = l
}

//Development sets the Development field on the package level config.
func Development(yes bool) {
	config.Development = yes
}

//Debug sets the Debug field on the package level config.
func Debug(yes bool) {
	config.Debug = yes
}

//UseMemory sets the UseMemory field on the package level config.
func UseMemory(yes bool) {
	config.UseMemory = yes
}
