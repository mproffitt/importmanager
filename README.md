# ImportManager

The idea of this program is to replace the bash script
[dtcp.bash](https://gist.github.com/mproffitt/d720a1a513cd3155316783a023f0edf2)

The original idea of `dtcp.bash` was to listen to an images "input" folder,
then sort the images to a new workbench location based on their filetype.

This program represents an idea to enhance that for managing multiple filetypes
based on rules defined against the mimetype for that file (group or category).

## Build and installation

### Build

```bash
go mod tidy
go build .
mv importmanager ~/bin
```

### Installation

- Create a working directory such as `~/.local/share/importmanager`
- [OPTIONAL] Create a `plugins` directory in the same location - you'll put your
  plugin scripts here.
- Copy [config.yaml](./config.yaml) and edit it to suit your requirements
- Copy [importmanager.service](./importmanager.service) to `~/.config/systemd/user`
- Enable the service with `systemctl --user enable importmanager.service`
- Start the service with  `systemctl --user start importmanager.service`

### Manual execution

To run the application manually, you must provide the config file as the only
argument

```bash
./importmanager -config config.yaml
```

## Configuration

### Paths

```yaml
paths:
  - path: /path/to/directory/to/watch
    processors:
      - [processor]
```

The main configuration section is the `paths` element. This contains a set of
paths to watch and [processors](#processors) to apply against that path.

For each path, the application will listen for the following events:

```golang
notify.All = notify.Create | notify.Remove | notify.Write | notify.Rename
```

See [notify](https://pkg.go.dev/github.com/rjeczalik/notify)

If directories start with `~/`, this is expanded to user home.

The configuration file is also watched using a slightly expanded set of notify
events to allow for automatic reloading of the file on change.

### Processors

```yaml
  processors:
    - type: mime/type
      handler: [builtin|scriptname]
      path: /path/to/destination/directory
      properties:
        property-1: property-value
        property-2: property-value
        ...
```

> **Warning**
>
> Processors can contain both wildcards and catagory level operations and whilst
> some recursion check is undertaken, it is not currently possible to detect
> deeply recursive moves. For example:
>
> This can be detected:
>
> ```yaml
> paths:
>   - path: /dir
>     processors:
>       - type: "*"
>         handler: move
>         path: /dir2
>   - path: /dir2
>     processors:
>       - type: "*"
>         handler: move
>         path: /dir
> ```
>
> But this may not be:
>
> ```yaml
> paths:
>   - path: /dir
>     processors:
>       - type: "image"
>         handler: move
>         path: /dir2
>   - path: /dir2
>     processors:
>       - type: "image/jpg"
>         handler: move
>         path: /dir3
>   - path: /dir3
>     processors:
>       - type: "image/jpg"
>         handler: move
>         path: /dir4
>   - path: /dir4
>     processors:
>       - type: "image/jpg"
>         handler: move
>         path: /dir
> ```

The dry run functionality tries to account for this by running the processors
multiple times but there may still be edge cases whereby recursion cannot be
detected.

to counteract this, try and keep your configuration to the fewest watch
locations possible and try not to move files to other watch locations unless
very strict rules are in place for handling files which are placed into that
location.

As an example of a good set of rules for processing images:

```yaml
paths:
  - path: ~/Downloads
    processors:
      # All images are moved to the image path where there are specific
      # processors defined for images
      - type: image
        path: ~/Images/sortme
        handler: move

      # Any executables that are downloaded are automatically moved to ~/bin
      - type: application/x-executable
        path ~/bin
        handler: install

      # Any Microsoft word documents are moved to the documents folder
      - type: application/vnd.openxmlformats-officedocument.wordprocessingml.document
        path: ~/Documents
        handler: move

  - path: ~/Images/sortme
    processors:
      # Any JPEG images are moved to a dated folder under ~/Images/jpg
      - type: image/jpg
        path: ~/Images/jpg/{{.date}}
        handler: move

      # Anything that is *not* an image is moved back to downloads
      - type: !image
        path: ~/Downloads
        handler: move
```

Each processor accepts the following options:

- `type` The mime type of the file to handle. This may be:
  - Final type (e.g. `image/x-canon-cr3`).
  - Parent type (e.g. `image/x-dcraw`)
  - Category type (e.g. `image`)
  - `*` This is placed at the same level as category and can be used when no
    other processor matches.
  - By placing a `!` in front of the type, that type is negated.

- `path` The destination path to write into. Each path may accept the following
  templated arguments
  - `{{.ext}}` The File extension (without the leading `.`)
  - `{{.date}}` This is populated with the last modification time of the file or
    in case of images, the `CreateDate` taken from ExifData if available. If
    exif data is used, this can be controlled through the property `exif-date`
    (see below).
  - `{{.ucext}}` This gives an upper case extension instead of the standard
    file lowercase extension variant (e.g. `cr2` becomes `CR2`).

- `handler` This is the handler to run for this type of file. By default, this
  should be one of the following built-in types:
  - `move` Moves the file from the watched directory to the destination
    specified by `path`
  - `copy` The same as move but leaves the original file in place
  - `delete` Simply deletes the file from disk. No warning is given.
  - `extract` Extracts the given file into `path` destination. By default this
    will auto-create a subfolder of the same name as the archive. This, in some
    instances may lead to paths which *stutter*, e.g. `example/example/`
    This method should also be used with care as certain file types may be sub-
    classed to the `application/x-zip` mime type.
  - `install`Moves the input file to the target location and sets the executable
    bit. Useful for shell scripts and/or app images that you want immediately
    available to use.

    > **Warning** `install` should not be used for package manager archives
    > (apt, rpm, etc.). This is because the install cannot and will never be
    > extended for `sudo` permissions. For this you must use a custom handler.
    > The reason for this is to prevent risk to your system from automatically
    > installing packages without verification or validation.

- `properties` Custom properties to control what happens to the file during
  handling. These are broadly split into 3 categories, pre processing, post
  processing and execution.

### Pre Processing properties

- `exif-date` For image processing only. Controls which exif data field take the
  date stamp from

### Post processing properties

- `chmod` Follows the BSD state. See `man chmod` for details.
- `chown` In the format `username:groupname`
- `setexec` Same as `chmod` with only `+x` provided.

### Execution properties

Execution properties control how each builtin method behaves. Each method may
implement its own properties.

#### `move` and `copy`

- `compare-sha` If a duplicate file is detected and this property is set, the
  function will calculate sha256 sums for both source and destination files.
  If the sha256 sums do not match, the filename will have a numeric integer
  attached to it and the copy will commence. For example, `mydoc.docx` would
  become `mydoc_1.docx`.

#### `install`

The install handler currently accepts the following additional properties

- `lowercase-destination` The final filename will be converted to lowercase
- `strip-extension` Strips the file extension from the final destination
  filename

`install` also inherits `compare-sha` as it uses `copy` as part of its
operation.

### Other configuration options

- `delayInSeconds` One of the drawbacks to `inotify` is its not possible to
  know when a file operation is completed. This setting controls how long to
  wait after the last received event before triggering the handler.

  In the sample file, this is set to 5 seconds.

- `cleanupZeroByte` Automatically delete files of 0 bytes in length.
- `pluginDirectory` Absolute path to the location to look for plugins used as
  handlers during processing.
- `bufferSize` the size of the worker pool buffer for each path being watched
  default 50

## Plugins

Rudimentary plugin support is available via scripts loaded into the
`pluginDirectory`.

At present only the following are supported:

- `python` This will use the system `python` command which depending on your
  system might be `python2`
- `sh` Shell scripts
- `bash` Bash scripts

> **Warning** No type or script safety is carried out on the *plugins* used by
> this application.  This can be dangerous and lead to your system being
> compromised. It is your responsibility to ensure all scripts in the plugin
_ location are safe. This program will only activate them.

The handler is given exactly 1 argument which is a JSON representation of the
details of that given handler:

Example:

```nohighlight
{
  "source": "/home/mproffitt/Descargas/IMG_0180.CR3",
  "destination": "/home/mproffitt/Imágenes/workbench/CR3/2022-11-11",
  "details": {
    "category": "image",
    "type": "image/x-canon-cr3",
    "subclass": [
      "image/x-dcraw"
    ],
    "extension": ".CR3"
  },
  "properties": {
    "include-date-directory": "true",
    "uppercase-extension-directory": "true"
  }
}
```

To enable the built in post processors for your script, the last line of output
should be the final location. This will be tested with `os.Stat` and if the path
exists, post-processing will take place against that location. Your user *must*
have write access to that location for post processing to work.

Sample plugins:

- [example.py](plugins/example.py)
- [example.bash](plugins/example.bash)

## TO DO

- Ambiguous type handling. For example, where multiple mime types share the
  same extension
- processors should accept a file extension as well as a mime
- Cross platform capability?

## Contributing

Fork the repo, create PRs, raise issues, buy me a bottle of non-alcoholic gin

Be creative.
