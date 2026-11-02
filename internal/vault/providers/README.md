# Provider integration boundary

`internal/vault` contains transport-neutral contracts and errors. Backends must
not import config, Cobra, clipboard, or TUI packages. `service` implements use
cases and dispatches optional operations by interface assertion.

`providers` is the composition layer: `Registry` maps configured provider names
to factories. Add a factory to `Builtins` when integrating a backend. Factories
receive configuration and a prompt callback; terminal handling stays in `cmd`.
Unknown providers return errors instead of being silently omitted from the TUI.

Required contract: `vault.Provider` plus setup inspection through `Managed`.
Optional contracts:

- `EntryCreator`: create-only publication. `service.Create` never falls back to
  overwrite-capable `Write`; unsupported providers return `ErrUnsupported`.
- `FolderCreator` / `FolderDeleter`: explicit empty-folder and recursive deletion.
- `Locker`: discard session-held identity state when the application exits.

For interactive reads, return `ErrPromptRequired` under `WithoutPrompt(ctx)`.
The root TUI retries with terminal ownership released. Never start a prompt
from a metadata operation or background search. Honor cancellation in listing.

Each named vault retains its own provider instance and browser view. Switching
changes the active Store form's service; quitting cancels all views' searches
and invokes every available Locker. Clipboard remains a service callback.

The root TUI supplies a provider-independent OperationRunner. Provider I/O runs
off the event loop with noninteractive context; only an explicit prompt-required
response retries with terminal ownership. Completion callbacks run on the UI
thread. Implementations must honor context cancellation and never mutate UI
state from work callbacks. Forms suppress duplicate submissions and discard
stale suggestion completions. New providers inherit this behavior without name
checks or provider-specific scheduling.

Provider tests should cover exact-value preservation, duplicate creates,
metadata-only listing, missing/uninitialized stores, cancellation, invalid paths,
and optional operations. Crypto/backends should return classified errors rather
than plaintext or raw subprocess output.
