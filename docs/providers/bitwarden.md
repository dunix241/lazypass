# Bitwarden and Vaultwarden

Install the official [Bitwarden CLI](https://bitwarden.com/help/cli/) (`bw`).
The same provider works with Bitwarden and self-hosted Vaultwarden servers.

## Configure a vault

For hosted Bitwarden:

```sh
lazypass vault init --provider bitwarden --name personal
```

For Vaultwarden or another Bitwarden region:

```sh
lazypass vault init --provider bitwarden --name work --server https://vault.example.com
```

Initialization prints a login command for that vault's isolated CLI directory.
Run it to sign in and complete any two-factor challenge, then open lazypass:

```sh
BITWARDENCLI_APPDATA_DIR="/path/printed/by/init" bw login
lazypass --vault personal vault
```

Lazypass asks for your master password when unlocking and keeps the session key
only for the current process. Each named vault has separate CLI account data.
Don't point two accounts at the same `storeDir`.

## Supported entries

Logins, cards, identities, secure notes, and SSH keys are supported. Browse personal
entries under `Personal/`, and shared entries under `Organizations/`, followed by
their organization and collection. Listing requires unlocking because Bitwarden
encrypts entry names too.

Shared entries can appear in multiple collections. Editing preserves their
memberships; deleting a shared entry removes it from every collection. Server
permissions apply. Manage organizations and collections in Bitwarden/Vaultwarden.

Folder names containing `/` appear as nested folders in lazypass. Editing a
value preserves additional URLs, custom fields, attachments, and TOTP settings.
Entries with duplicate names in the same folder can be listed, but ambiguous
read/edit/delete operations are rejected; rename them in Bitwarden first.

Changes are sent through `bw` to your server. Deleting an entry uses Bitwarden's
normal trash behavior. Folder deletion removes supported personal login items
below it before deleting folders; it is not an atomic operation. Other item
types cause folder deletion to be rejected so those items are not affected.

Run `lazypass --vault personal vault sync` to explicitly refresh the CLI's local
data. Duplicate-name checking uses that local snapshot; another client can still
create an identically named entry concurrently.

## Account and recovery

Account login data remains in the isolated Bitwarden CLI directory. Master
passwords and unlock session keys are not saved in lazypass configuration.
Use Bitwarden's account recovery, export, and backup tools for your account.

[Back to lazypass](../../README.md)
