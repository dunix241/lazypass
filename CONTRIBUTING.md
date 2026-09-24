# Contributing

Go source is formatted exclusively with the standard `gofmt` tool. Do not
manually align Go code or use an alternate formatter.

Before opening a pull request, run:

```sh
make fmt
make fmt-check
make test
```

The release workflow runs `make fmt-check`, so unformatted Go files cannot be
released.

## Demo recording

The README demo is recorded as text with asciinema and converted to GIF, so it
stays pixel-perfect. Install with `sudo pacman -S asciinema` and `agg` from the
AUR (or `cargo install --git https://github.com/asciinema/agg`).

```sh
asciinema rec --window-size 100x28 --idle-time-limit 2 assets/demo.cast
```

This opens a shell: run the TUI, demonstrate it, then press `Ctrl+d` to stop.
Use a large terminal font before recording so the text stays readable.
Replay with `asciinema play assets/demo.cast`, then convert:

```sh
agg assets/demo.cast assets/demo.gif
```

Commit the `.gif`, referenced from the README as `![Demo](assets/demo.gif)`.
