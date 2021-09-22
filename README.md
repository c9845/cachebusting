## Introduction:
This package provides tooling for creating and using cache busting copies of static files. 

## Details:
Cache busting is performed *at runtime* by prepending a partial hash to each of your source/original file's name, storing a copy of the file for use in responding to requests, and providing you a key-value map of the original file name to the cache busting file name (i.e.: script.min.js -> A1B2C3D4.script.min.js). You would take the key-value map and handle building your HTML files/templates with the correct filename.  

Usage of this package is based upon a few assumptions:
1) You know the non-cache busting file's name and use that name in your HTML files/templates. In other words, your source/original file's name doesn't change.
1) You are okay with cache busting being performed by renaming of the source/original files.
1) You have some manner of performing replacement of text in your HTML files/templates (i.e.: you use the `html/templates` package).

Further details:
- Works with on-disk or embedded source template files.
- Store configuration in-package (globally) or elsewhere (dependency injection).
- Store the cache busting copies of your original files on disk or in memory (in memory only for embedded files).

## Getting Started:
With your original static files in a directory structure similar to as follows:
```
/path/to/static/
├─ css/
│  ├─ styles.min.css
├─ js/
│  ├─ script.min.js
```

1) Build the data structures for your static files by providing the local on-disk path and the path used on your website/app. Make note of the separators you use!
```golang
css := NewStaticFile(filepath.Join("/", "path", "to", "static", "css", "styles.min.css"), path.Join("/", "static", "css", "styles.min.css"))
```

1) Initialize your configuration using `NewConfig()` or `NewOnDiskConfig` if you want to store your configuration elsewhere; or `DefaultConfig` or `DefaultOnDiskConfig` if you want to use the globally stored configuration.
```golang
err := NewOnDiskConfig(css)
if err := nil {
    log.Fatal(err)
    return
}
```

1) Call `Create()` to validate your configuration and perform the creation of the cache busting files and data.

1) Retrieve the key-value map of original to cache busting file names via `GetFilenamePairs()`. Pass this value to your HTML files/templates and handle replacement of the original files. Example code to handle replacement if you use the `html/templates` package is noted as follows:

```html
<html>
  <head>
    {{$originalFile := "styles.min.css"}}
	{{$cacheBustFiles := .CacheBustFiles}} {{/* the key-value map retrieved via GetFilenamePairs */}}

	{{/*If the key "styles.min.css" exists in $cacheBustFiles, then the associated cache-busted filename will be returned as {{.}}. */}}
	{{with index $cacheBustFiles $originalFile}}
	  {{$cacheBustedFile := .}}
	  <link rel="stylesheet" href="/static/css/{{$cacheBustedFile}}">
    {{else}}
      <link rel="stylesheet" href="/static/css/{{$originalFile}}">
    {{end}}
  </head>
</html>
```

## Using Embedded Files:
This package can work the files embedded via the `embded` package. You *must* have already "read" the embedded files using code similar to below *prior* to providing the `embed.FS` object to this package. Note that the paths *must* use a forward slash separator!

```golang
package main

//go:embed path/to/templates
var embeddedFiles embed.FS

func init() {
    css = NewStaticFile(path.Join("path", "to", "static", "css", "styles.min.css"), path.Join("/", "static", "css", "styles.min.css"))
    c = NewEmbeddedConfig(embeddedFiles, css)
    err := c.Build()
    if err != nil {
        log.Fatal(err)
        return
    }
}
```

## Serving Files:
Based on the configuration of this package, the files you want to serve could be located in one of three locations:
- On disk.
- Memory.
- Embedded.

Due to this, determining how to serve a request is a bit more difficult then just serving an on-disk directory using `http.FileServer`. There is an example HTTP handler provided that may work for you but is very strict on it's source directory layout (see below). This handler is designed to serve  cache busting files, non-cache busting files, and locally stored third party libraries (i.e.: bootstrap) where certain files may be stored in memory and other are stored on disk or embedded.

Expected directory structure for the example HTTP handler:
```
/path/to/website/
├─ static/
│  ├─ css
│  │  ├─ styles.min.css
│  ├─ js
│  │  ├─ script.min.js
```

## Copies of Source Files:
During the `Create()` process, copies of each source file are created and stored either on disk or in memory. A copy is created, versus just using a new filename to point to the original source file, so that there is a reduced chance of serving a file whose contents have changed since the time the hash has be calculated.

The copy of embedded files are always stored in memory since you cannot write to the embedded filesystem. You have the option of storing the copy of on disk files in memory for times when your app does not have write access to the system it is running or just personal choice.
