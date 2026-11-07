# lazypass

Generate and manage passwords from your terminal. Open the interactive app, or
generate passwords directly from the command line. Your preferences are saved
for next time.

## Features

- Customizable password length and character sets, with a live strength meter.
- Encrypted vaults for generated passwords and values you already have.
- Multiple vaults, folders, recursive search, and entry editing.
- Built-in themes and a custom-theme editor.
- Clipboard support locally and over SSH.
- Plain text and JSON output for scripts.

## Demo

![Demo](assets/demo.gif)

## Install

```sh
curl -fsSL https://github.com/dunix241/lazypass/releases/latest/download/install.sh | sh
```

The installer supports Linux, macOS, and Android ARM64 in Termux and installs into `~/.local/bin`.
Windows binaries are available on the [releases page](https://github.com/dunix241/lazypass/releases/latest).
To build from source, run `make build`; the binary is
written to `.build/lazypass`.

## Usage

### TUI mode

Open the interactive app to generate passwords and manage your vaults:

```sh
lazypass                         # open the app
lazypass vault                   # open your vaults
```

Available actions are shown in the app as you use it.

### CLI mode

Generate passwords directly for shell commands and scripts:

```sh
lazypass generate --length 32 --symbols
lazypass generate --format json --count 5
```

Use `--help` on any command for its options. Explicit generation options become your defaults;
add `--no-save` for a one-off command.

### Vaults

Choose a provider to get started:

- [age](docs/providers/age.md): built-in encryption with a passphrase-protected identity.
- [pass](docs/providers/pass.md): use the Unix password manager and your GPG keys.
- [Bitwarden / Vaultwarden](docs/providers/bitwarden.md): connect with the official Bitwarden CLI.

You can keep multiple named vaults and switch between them in the app. For CLI
commands, select one with `--vault <name>`:

```sh
lazypass --vault personal vault list
lazypass --vault personal vault copy personal/email
```

### Configuration

Settings are stored in `~/.config/lazypass/config.yaml`. Use `lazypass config path`
to find your file or `lazypass config show` to inspect it. A different config
can be selected with `--config <file>` or `LAZYPASS_CONFIG`.

### Clipboard over SSH

SSH copy uses your terminal's clipboard support (OSC 52). Use an interactive
SSH session and a terminal that permits clipboard writes. For remote tmux,
enable `set -g allow-passthrough on`; a local tmux layer also needs clipboard
forwarding, such as `set -g set-clipboard on`.

For local Linux clipboard support, install `wl-copy` on Wayland or `xclip` on
X11. macOS and Windows use native clipboard tools.

### Updates

Update a standalone installation with `lazypass update`. Package-managed
installations show guidance for their package manager. Rerun the installer
command to upgrade an existing installation.

Lazypass notifies you when a new release is available. Check manually with
`lazypass update check`. Configure automatic behavior with:

```yaml
updates:
  mode: notify
  cacheMaxAge: 1h
```

Use `off` to disable automatic checks, `notify` (the default) for notifications,
or `auto` to install updates automatically for standalone installations.
Other installation methods receive notifications. Restart lazypass after an
automatic update to use the new version.

`cacheMaxAge` controls how long a release lookup is reused across startups.
Use a positive duration such as `15m`, `1h`, or `2h30m`. Checks run on TUI startup;
manual update commands always fetch the latest release.

### Troubleshooting

Set `LAZYPASS_DEBUG=1` to record failure diagnostics in
`~/.local/state/lazypass/debug.log` (or `$XDG_STATE_HOME/lazypass/debug.log`).
