# pass vault

Use an existing [pass](https://www.passwordstore.org/) store or create one with
your GPG key.

## Install dependencies

Install pass, GPG, and terminal pinentry:

| Distribution | Command |
| --- | --- |
| Arch / Manjaro | `sudo pacman -S pass gnupg pinentry` |
| Debian / Ubuntu / Linux Mint | `sudo apt update && sudo apt install pass gnupg pinentry-curses` |
| Fedora | `sudo dnf install pass gnupg2 pinentry` |
| openSUSE | `sudo zypper install password-store gpg2 pinentry` |
| Alpine | `sudo apk add pass gnupg pinentry` |

## Create a vault

Find your GPG key's fingerprint:

```sh
gpg --list-secret-keys --keyid-format LONG
```

If you don't have a key, create one with `gpg --full-generate-key`. Then initialize:

```sh
lazypass vault init --provider pass --name work --recipient YOUR_GPG_FINGERPRINT
lazypass vault
```

This initializes the store and adds it to your configuration. Use
`--store-dir /path/to/store` for a custom location. Otherwise pass uses
`PASSWORD_STORE_DIR` or `~/.password-store`.

## Use an existing store

Add it to your config without initializing it again:

```yaml
vaults:
  - name: work
    provider: pass
    storeDir: ~/.password-store
```

The store must already be initialized, and GPG must have access to the secret
key needed to decrypt its entries. Entry and folder names remain visible on disk.

## SSH access

Install the dependencies above on the remote machine so GPG can ask for a
passphrase in your SSH session.

Lazypass automatically configures GPG to use the current terminal. Pinentry can
show a graphical prompt locally and a terminal prompt over SSH.

Your secret key must be available on the remote machine or through a forwarded
GPG agent.

## Sync and backup

Lazypass stores type-only records under `.lazypass/types/` at the store root and
commits them when the store is Git-enabled. They contain entry paths and types,
not passwords or other secret fields. Sync this directory with your entries.
To index entries created before this feature or repair metadata after changes
made in another client:

```sh
lazypass --vault work vault metadata rebuild
```

This decrypts entries to discover their types and can be interrupted. Unreadable
entries are counted as failures; missing metadata does not prevent using them.

For a Git-enabled store, sync explicitly:

```sh
lazypass --vault work vault sync
```

Back up your password store and GPG secret key, and retain the key's passphrase.
Empty folders are not tracked by Git until they contain entries.

[Back to lazypass](../../README.md)
