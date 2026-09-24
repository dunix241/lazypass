# lazypass-bin AUR Release

Use this directory to update the separate `lazypass-bin` AUR repository after a
GitHub release is published.

1. Copy `PKGBUILD.template` to that repository as `PKGBUILD`.
2. Replace `pkgver` with the release version, without the `v` prefix.
3. Download the Linux `x86_64` and `arm64` release archives and set each
   `sha256sums_*` value from `sha256sum` output.
4. Run `makepkg --printsrcinfo > .SRCINFO`, then test with `makepkg -si`.

The archive names and binary paths are defined in `.goreleaser.yaml`; update
this template if those release settings change.
