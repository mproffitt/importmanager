# ImportManager

The idea of this program is to replace the bash script
[dtcp.bash](https://gist.github.com/mproffitt/d720a1a513cd3155316783a023f0edf2)

The original idea of `dtcp.bash` was to listen to an images "input" folder,
then sort the images to a new workbench location based on their filetype.

This program represents an idea to enhance that for managing multiple filetypes
based on rules defined against the mimetype for that file (group or category).

## Execution

Create a [config.yaml](./config.yaml) using the configuration settings below
as a guide then run as:

```bash
./importmanager -config config.yaml
```

You may also define this as a `systemd --user` service.

## Configuration

There are two primary sections to the configuration file, `watch` and
`processors`

- `watch` A list of directories to watch for events.
  Primarily, the following system events are listened for:

  ```golang
  notify.All | notify.InCloseWrite
  ```

  See [notify](https://pkg.go.dev/github.com/rjeczalik/notify)

  If directories start with `~/`, this is expanded to user home.

- `processors` This section details how to process each type of file using the
  mime type of the file as a reference.

### Processors

Processors are automatically reloaded. This means that even with the application
running, new processors can be added and automatically take effect without
having to restart the application. This is not (currently) true for `watch` or
other configuration settings.

Each processor accepts the following options:

- `type` The mime type of the file to handle. This may be:
  - Final type (e.g. `image/x-canon-cr3`)
  - Parent type (e.g. `image/x-dcraw`)
  - Category type (e.g. `image`)
- `path` The destination path to write into. Each path may accept the following
  templated arguments
  - `{{.ext}}` The File extension (without the leading `.`)
  - `{{.date}}` This is populated with the last modification time of the file or
    in case of images, the `CreateDate` taken from ExifData if available. If
    exif data is used, this can be controlled through the property `exif-date`
    (see below).
  - `{{.ucext}}` This gives an upper case extenstion instead of the standard
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
    > extended for `sudo` permissions. Future iterations of this application
    > will support custom handlers. If you require this behavior, you'll be
    > welcome to write your own handler.
- `properties` Custom properties to control what happens to the file during
  handling. These are broadly split into 3 catagories, pre processing, post
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

#### `install`

The install handler currently accepts the following additional properties

- `lowercase-destination` The final filename will be converted to lowercase
- `strip-extension` Strips the file extension from the final destination filename

### Other configuration options

- `delayInSeconds` One of the drawvbacks to `inotify` is its not possible to
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

Sample plugins:

- [example.py](plugins/example.py)
- [example.bash](plugins/example.bash)

## TO DO

- Include mime type descriptions from other locations than `usr/share/mime`
  - `/usr/local/share/mime`
  - `~/.local/share/mime`
- Cross platform capability?

## Contributing

Fork the repo, create PRs, raise issues, buy me a bottle of non-alcoholic gin

Be creative.
