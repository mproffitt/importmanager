# ImportManager

> **Note** This is a work in progress.

The idea of this program is to replace the bash script
[dtcp.bash](https://gist.github.com/mproffitt/d720a1a513cd3155316783a023f0edf2)

The original idea of `dtcp.bash` was to listen to an images "input" folder,
then sort the images to a new workbench location based on their filetype.

This program represents an idea to enhance that for managing multiple filetypes
based on rules defined against the mimetype for that file (group or category).

## Configuration

There are two primary sections to the configuration file, `watch` and
`processors`

- `watch` A list of directories to watch for events.
  Primarily, the following system events are listened for:

  ```golang
  notify.All | notify.InCloseWrite
  ```

  If directories start with `~/`, this is expanded to user home.

- `processors` This section details how to process each type of file using the
  mime type of the file as a reference.

### Processors

Processors are automatically reloaded. This means that even with the application
running, new processors can be added and automatically take effect without
having to restart the application. This is not (currently) true for `watch` or
other configuration settings.

> **Note** future iterations of this application will allow for user defined
> scripted processors to be supplied.

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

    > **Warning** Install should not be used for package manager archives
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

## TO DO

There are mainly two outstanding items to complete for this applicartion

1. Currently the application tries to immediately operate on all files that
   appear in the `watch` locations. When copying images (e.g. from a camera)ç
   this may result in a large number of files being handled simultaneously

   One thing that needs to be implemented is controllable buffering to prevent
   overloading the system during bulk operations.

1. An extensible plugin system.

   Not every action can or should be written in golang. I'd like to be able to
   fork out to other languages for handling certain types of file; namely shell
   and python for operations such as image manipulation. This would mean that
   the program doesn't need to he recompiled just because someone wants a new
   handler for unknown filetype X.

Other outstanding tasks:

- Include mime type descriptions from other locations than `usr/share/mime`
  - `/usr/local/share/mime`
  - `~/.local/share/mime`
- Cross platform capability?
- Testing? (or not)...
- I'll think of something else I'm sure.

## Contributing

Fork the repo, create PRs, raise issues, buy me a bottle of non-alcoholic gin

Be creative.