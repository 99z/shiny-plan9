# Go exp + Plan 9 driver for Shiny

This repo is a snapshot of commit [`c5b29ce`](https://go-review.googlesource.com/c/exp/+/37621) made to Go's experimental packages repository.

It was not merged into the master branch but adds a Plan 9 driver for the Shiny GUI framework.

My specific use-case for this is drawing to the screen on Plan 9 to make my Game Boy emulator [halken](https://github.com/nicoNaN/halken) run on it.

Thanks to [David du Colombier](https://github.com/0intro) for providing me with this info.

# Original README

This subrepository holds experimental and deprecated (in the "old"
directory) packages.

The idea for this subrepository originated as the "pkg/exp" directory
of the main repository, but its presence there made it unavailable
to users of the binary downloads of the Go installation. The
subrepository has therefore been created to make it possible to "go
get" these packages.

Warning: Packages here are experimental and unreliable. Some may
one day be promoted to the main repository or other subrepository,
or they may be modified arbitrarily or even disappear altogether.

In short, code in this subrepository is not subject to the Go 1
compatibility promise. (No subrepo is, but the promise is even more
likely to be violated by go.exp than the others.)

Caveat emptor.
