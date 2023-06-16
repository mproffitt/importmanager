# CHANGELOG

## [Unreleased]

- Add dry run functionality. See README.md for details
- Add functionality to negate types
- Add `compare-sha` functionality

## [v0.1.0]

This release is the first real version of my ImportManager

### Features

- :hand: Handle any file group by its mime-type, parent type or category.
- :watch: Multi-path management. Define paths and processors that can chain
  together to put files exactly where you need them
- :electric_plug: Define your own plugins. Write scripts in :snake: `python`
  or :shell: `shell` to bring your own handlers to the game

### :bug: Bugs

It's possible to ping-pong files between locations on disk as there's no
validation on the path processors.

- Solution is to ensure if one path is writing to another watched path,
  wildcard types should not be defined.
