#!/bin/sh
set -eu
umask 077

RELEASES_URL=https://github.com/dunix241/lazypass/releases
RECEIPT_LIMIT=8192

die() {
    printf 'lazypass: %s\n' "$*" >&2
    exit 1
}

usage() {
    cat <<'EOF'
Usage: sh install.sh [options]

Options:
  --version vX.Y.Z   Install a specific stable release (default: latest)
  --install-dir DIR Install into an absolute directory (default: ~/.local/bin)
  --adopt           Explicitly replace an unrecognized installation
  -h, --help        Show this help
EOF
}

parse_args() {
    version=
    install_dir=
    adopt=false

    while [ "$#" -gt 0 ]; do
        case "$1" in
            --version | --install-dir)
                if [ "$#" -lt 2 ] || [ -z "$2" ]; then
                    die "missing value for $1"
                fi
                case "$1" in
                    --version) version=$2 ;;
                    --install-dir) install_dir=$2 ;;
                esac
                shift 2
                ;;
            --adopt)
                adopt=true
                shift
                ;;
            -h | --help)
                usage
                exit 0
                ;;
            *) die "unknown option: $1" ;;
        esac
    done

    [ -n "${HOME:-}" ] || die 'HOME is required'
    install_dir=${install_dir:-$HOME/.local/bin}
    state_dir=${XDG_STATE_HOME:-$HOME/.local/state}/lazypass

    [ -z "$version" ] || valid_version "$version" || die 'expected stable version vX.Y.Z'
    case "$install_dir" in
        /*) ;;
        *) die 'install directory must be absolute' ;;
    esac
    case "$state_dir" in
        /*) ;;
        *) die 'XDG_STATE_HOME must be absolute' ;;
    esac
}

valid_version() {
    printf '%s\n' "$1" | LC_ALL=C grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'
}

require_tools() {
    for tool in curl tar awk grep sed mktemp uname mkdir rmdir rm cat chmod mv wc cut; do
        command -v "$tool" >/dev/null 2>&1 || die "required tool missing: $tool"
    done

    if command -v sha256sum >/dev/null 2>&1; then
        hash_tool=sha256sum
    elif command -v shasum >/dev/null 2>&1; then
        hash_tool=shasum
    else
        die 'sha256sum or shasum is required'
    fi
}

sha256() {
    case "$hash_tool" in
        sha256sum) hash_output=$(sha256sum <"$1") || die 'could not hash file' ;;
        shasum) hash_output=$(shasum -a 256 <"$1") || die 'could not hash file' ;;
    esac
    printf '%s\n' "${hash_output%% *}"
}

detect_platform() {
    os=$(uname -s)
    case "$os" in
        Linux | Darwin) ;;
        *) die "unsupported operating system: $os" ;;
    esac

    arch=$(uname -m)
    if [ "$os" = Darwin ] && [ "$(sysctl -in sysctl.proc_translated 2>/dev/null || :)" = 1 ]; then
        arch=arm64
    fi
    case "$arch" in
        x86_64 | amd64) arch=x86_64 ;;
        arm64 | aarch64) arch=arm64 ;;
        *) die "unsupported architecture: $arch" ;;
    esac
    asset=lazypass_${os}_${arch}.tar.gz
}

cleanup() {
    status=$?
    trap - 0
    set +e
    [ -z "$work_dir" ] || rm -rf "$work_dir"
    [ -z "$binary_stage" ] || rm -f "$binary_stage"
    [ -z "$receipt_stage" ] || rm -f "$receipt_stage"
    [ -z "$lock_dir" ] || rmdir "$lock_dir"
    exit "$status"
}

prepare_workspace() {
    work_dir=
    binary_stage=
    receipt_stage=
    lock_dir=
    trap cleanup 0
    trap 'exit 130' INT
    trap 'exit 143' TERM
    trap 'exit 129' HUP
    work_dir=$(mktemp -d)
}

curl_https() {
    curl --proto '=https' --proto-redir '=https' --tlsv1.2 \
        --fail --silent --show-error --location \
        --connect-timeout 10 --max-time 120 --retry 1 "$@"
}

resolve_version() {
    if [ -z "$version" ]; then
        release_url=$(curl_https --output /dev/null --write-out '%{url_effective}' "$RELEASES_URL/latest")
        version=${release_url##*/}
        valid_version "$version" || die 'could not resolve latest stable release'
    fi
}

query_owner() {
    command -v "$1" >/dev/null 2>&1 || return 0

    if "$@" >/dev/null 2>&1; then
        die 'destination is package-managed; update with your package manager'
    else
        code=$?
        [ "$code" -eq 1 ] || die "package ownership query failed: $1"
    fi
}

check_package_ownership() {
    case "$target" in
        /nix/store/* | */Cellar/*) die 'use your package manager to update this installation' ;;
    esac

    [ -e "$target" ] || return 0
    case "$os" in
        Linux)
            query_owner pacman -Qoq "$target"
            query_owner dpkg-query -S "$target"
            query_owner rpm -qf "$target"
            query_owner apk info --who-owns "$target"
            ;;
    esac
}

prepare_destination() {
    mkdir -p "$install_dir"
    install_dir=$(CDPATH='' cd -P "$install_dir" && pwd -P)
    target=$install_dir/lazypass

    [ ! -L "$target" ] || die 'destination is a symlink; use the resolved installation directory'
    [ ! -e "$target" ] || [ -f "$target" ] || die 'destination is not a regular file'
    check_package_ownership

    mkdir "$install_dir/.lazypass-install.lock" 2>/dev/null ||
        die 'installation is locked; if no installer is running, remove .lazypass-install.lock from the install directory'
    lock_dir=$install_dir/.lazypass-install.lock

    # Encode characters individually: awk gsub replacement escaping varies by implementation.
    json_path=$(printf '%s\n' "$target" | LC_ALL=C awk '
        NR > 1 || /[[:cntrl:]]/ { exit 1 }
        {
            for (i = 1; i <= length($0); i++) {
                c = substr($0, i, 1)
                if (c == "\\" || c == "\"") printf "\\"
                printf "%s", c
            }
        }
    ') || die 'unsupported control character in destination'
}

write_receipt() {
    printf '{"version":1,"method":"script","executable":"%s","release":"%s","sha256":"%s"}\n' \
        "$json_path" "$1" "$2"
}

read_installed_version() {
    old_version=
    receipt_file=$state_dir/install.json
    [ -f "$receipt_file" ] || return 0
    [ "$(wc -c <"$receipt_file")" -le "$RECEIPT_LIMIT" ] || return 0

    old_version=$(sed -n 's/.*"release":"\(v[0-9]*\.[0-9]*\.[0-9]*\)".*/\1/p' "$receipt_file")
    if ! valid_version "$old_version" ||
        [ "$(write_receipt "$old_version" "$old_hash")" != "$(cat "$receipt_file")" ]; then
        old_version=
    fi
}

is_upgrade() {
    LC_ALL=C awk -v old="$old_version" -v new="$version" '
        BEGIN {
            sub(/^v/, "", old)
            sub(/^v/, "", new)
            split(old, a, ".")
            split(new, b, ".")
            for (i = 1; i <= 3; i++) {
                if (length(b[i]) != length(a[i])) exit !(length(b[i]) > length(a[i]))
                if ("x" b[i] != "x" a[i]) exit !("x" b[i] > "x" a[i])
            }
            exit 1
        }
    '
}

check_existing_installation() {
    old_hash=
    [ -f "$target" ] || return 0

    old_hash=$(sha256 "$target")
    read_installed_version
    [ -n "$old_version" ] || [ "$adopt" = true ] ||
        die 'existing installation is unrecognized; use --adopt to explicitly replace it'

    if [ -n "$old_version" ]; then
        if [ "$version" = "$old_version" ]; then
            printf 'Already installed: %s\n' "$version"
            exit 0
        fi
        is_upgrade || die 'downgrades are not supported'
    fi
}

download_release() {
    printf 'Downloading lazypass %s (%s/%s)…\n' "$version" "$os" "$arch"
    release_url=$RELEASES_URL/download/$version
    curl_https --output "$work_dir/archive.tar.gz" "$release_url/$asset"
    curl_https --output "$work_dir/checksums" "$release_url/lazypass_checksum_sha256.txt"

    expected=$(awk -v name="$asset" '
        $2 == name || $2 == "*" name {
            count++
            digest = $1
            if (NF != 2) malformed = 1
        }
        END {
            if (malformed || count != 1 || length(digest) != 64 || digest ~ /[^0-9a-f]/) exit 1
            print digest
        }
    ' "$work_dir/checksums") || die 'missing, duplicate, or malformed checksum'

    [ "$(sha256 "$work_dir/archive.tar.gz")" = "$expected" ] || die 'archive checksum mismatch'
}

extract_binary() {
    # Stream only the unique regular executable; never extract archive paths.
    tar -tzf "$work_dir/archive.tar.gz" >"$work_dir/members"
    count=$(awk '$0 == "lazypass" { n++ } END { print n + 0 }' "$work_dir/members")
    [ "$count" -eq 1 ] || die 'archive must contain exactly one lazypass executable'

    tar -tvzf "$work_dir/archive.tar.gz" lazypass >"$work_dir/type"
    [ "$(cut -c1 "$work_dir/type")" = '-' ] || die 'archive executable must be a regular file'
    tar -xOzf "$work_dir/archive.tar.gz" lazypass >"$work_dir/lazypass"
    [ -s "$work_dir/lazypass" ] || die 'empty executable'
}

check_destination_unchanged() {
    [ ! -L "$target" ] || die 'destination changed during installation'
    if [ -n "$old_hash" ]; then
        if [ ! -f "$target" ] || [ "$(sha256 "$target")" != "$old_hash" ]; then
            die 'destination changed during installation'
        fi
    else
        [ ! -e "$target" ] || die 'destination appeared during installation'
    fi
    check_package_ownership
}

publish_installation() {
    digest=$(sha256 "$work_dir/lazypass")
    mkdir -p "$state_dir"
    receipt_stage=$(mktemp "$state_dir/.install.XXXXXXXX")
    write_receipt "$version" "$digest" >"$receipt_stage"

    binary_stage=$(mktemp "$install_dir/.lazypass.XXXXXXXX")
    cat "$work_dir/lazypass" >"$binary_stage"
    chmod 755 "$binary_stage"

    check_destination_unchanged
    mv -f "$binary_stage" "$target"
    binary_stage=
    mv -f "$receipt_stage" "$state_dir/install.json" ||
        die 'binary installed, but receipt could not be saved; next update requires --adopt'
    receipt_stage=
}

main() {
    parse_args "$@"
    require_tools
    detect_platform
    prepare_workspace
    prepare_destination
    resolve_version
    check_existing_installation
    download_release
    extract_binary
    publish_installation

    printf 'Installed lazypass %s to %s\n' "$version" "$target"
    case ":${PATH:-}:" in
        *":$install_dir:"*) ;;
        *) printf 'Add %s to your PATH.\n' "$install_dir" ;;
    esac
}

main "$@"
