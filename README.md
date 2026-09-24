# lazypass

Interactive local password generator for the terminal. `lazypass` opens an interactive TUI; `lazypass generate` prints passwords for scripts. Whatever you change is saved and used again next time.

## Features

- TUI and CLI in one binary; scripts get plain stdout.
- Live strength meter: entropy in bits (`length × log2(pool size)`), labeled Weak below 40, Fair below 60, Good below 80, Strong at 80 and above.
- Options save automatically and are reused next time; inspect with `config path` / `config show`.
- Generation uses the OS cryptographic random source via `crypto/rand`, so every character is equally likely. One character per enabled class is guaranteed.
- Ambiguous-character filter (`I l 1 O 0`) and custom symbol set.
- Copy goes straight to the clipboard (`wl-copy`, `xclip`, or native) and is never printed.
- JSON output (`generate --format json`) with password, length, entropy bits, and strength.
- Layout adapts to narrow or short terminals (minimum 32×10), mouse input supported.

## Demo

![Demo](assets/demo.gif)

## Usage

```
lazypass              # TUI
lazypass generate     # one password to stdout
```

### TUI

| Key                     | Action                                                           |
| ----------------------- | ---------------------------------------------------------------- |
| `←` / `→`               | decrease / increase length                                       |
| `+` / `-`               | increase / decrease length                                       |
| `0`–`9`                 | type length directly (while length field is focused)             |
| `Backspace`             | delete a typed digit                                             |
| `U` `L` `N` `S` `E`     | toggle uppercase, lowercase, numbers, symbols, exclude-ambiguous |
| `Space` / `Enter`       | toggle or activate focused item                                  |
| `Tab` / `↓` / `j`       | next item                                                        |
| `Shift+Tab` / `↑` / `k` | previous item                                                    |
| `r`                     | regenerate                                                       |
| `c`                     | copy to clipboard                                                |
| `q` / `Esc`             | save and quit                                                    |

### CLI

```
lazypass generate --length 32 --symbols --count 5
lazypass generate --format json --copy
lazypass --no-tui -l 24 --no-numbers
```

Available options: `-l/--length` (4–256), `--upper` / `--no-upper`, `--lower` / `--no-lower`, `--numbers` / `--no-numbers`, `--symbols` / `--no-symbols`, `--exclude-ambiguous`, `--symbol-set`, `--count`, `--format text|json`, `--copy`, `--no-save`. Piping output also skips the TUI (same as `--no-tui`).

Options you pass explicitly are saved as your new defaults; add `--no-save` to keep it a one-off. Contradictory pairs like `--upper --no-upper` are rejected.

### Saved options

Options live in `~/.config/lazypass/config.yaml` (XDG). `lazypass config path` prints the resolved path and `lazypass config show` prints the saved options. Use `--config <file>` or `LAZYPASS_CONFIG` to point elsewhere.

If generate/strength output matters in automation, `--format json` emits one JSON object per line with `password`, `length`, `entropyBits`, and `strength`.

## Install

```sh
mkdir -p ~/.local/bin && OS=$(uname -s) && ARCH=$(uname -m | sed 's/aarch64/arm64/') && curl -fsSL "https://github.com/dunix241/lazypass/releases/latest/download/lazypass_${OS}_${ARCH}.tar.gz" | tar -xz -C ~/.local/bin lazypass
```

Make sure `~/.local/bin` is on your `PATH`. Alternatively:

```
go install github.com/dunix241/lazypass@latest
```

Or build from source: `make build` (binary lands in `.build/lazypass`). Prebuilt archives for Linux, macOS, and Windows (x86_64, arm64) are attached to GitHub releases. For Arch, `packaging/aur` documents the `lazypass-bin` package update flow.

Clipboard copy needs `wl-copy` (Wayland) or `xclip` (X11) on Linux; macOS and Windows use their native tooling.

Copied passwords go straight to the clipboard and are never printed to stdout.
