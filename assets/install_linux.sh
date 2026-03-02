#!/bin/bash

set -euo pipefail

APP_NAME="debrief"
BASEDIR=$(dirname "$(realpath "$0")")
PREFIX=${PREFIX:-/usr/local}
BIN_DIR="$PREFIX/bin"
APP_DIR="$PREFIX/share/applications"
ICON_DIR="$PREFIX/share/icons/hicolor/256x256/apps"

usage() {
    echo "Usage: $0 [--uninstall]"
    echo ""
    echo "Installs $APP_NAME to $PREFIX."
    echo "Set PREFIX env var to change install location."
    exit 0
}

check_root() {
    if [ "$EUID" -ne 0 ]; then
        echo "This script requires root access to install the package."
        echo "Please enter your password to continue:"
        exec sudo "$0" "$@"
    fi
}

check_dependencies() {
    local missing=0
    for cmd in install chmod rm dirname realpath; do
        if ! command -v "$cmd" &>/dev/null; then
            echo "Error: required command not found: $cmd" >&2
            missing=1
        fi
    done
    if [ "$missing" -ne 0 ]; then
        echo "Please install the missing dependencies and try again." >&2
        exit 1
    fi
}

check_runtime_libs() {
    if ! command -v ldconfig &>/dev/null; then
        echo "Warning: ldconfig not found, skipping runtime library check." >&2
        return
    fi

    local missing=0
    local libs=(
        libwayland-client
        libX11
        libxkbcommon-x11
        libGLESv2
        libEGL
        libXcursor
        libvulkan
    )

    for lib in "${libs[@]}"; do
        if ! ldconfig -p | grep -q "$lib"; then
            echo "Error: missing runtime library: $lib" >&2
            missing=1
        fi
    done

    if [ "$missing" -ne 0 ]; then
        echo "" >&2
        echo "Install the missing libraries and try again." >&2
        echo "  Debian/Ubuntu: sudo apt install libwayland-client0 libx11-6 libxkbcommon-x11-0 libgles2 libegl1 libxcursor1 libvulkan1" >&2
        echo "  Fedora/RHEL:   sudo dnf install libwayland-client libX11 libxkbcommon-x11 mesa-libGLESv2 mesa-libEGL libXcursor vulkan-loader" >&2
        echo "  Arch:          sudo pacman -S wayland libx11 libxkbcommon-x11 mesa libxcursor vulkan-icd-loader" >&2
        exit 1
    fi
}

validate_source_files() {
    local missing=0
    for f in \
        "$BASEDIR/$APP_NAME" \
        "$BASEDIR/desktop-assets/$APP_NAME.desktop" \
        "$BASEDIR/appicon.png"; do
        if [ ! -f "$f" ]; then
            echo "Error: missing required file: $f" >&2
            missing=1
        fi
    done
    if [ "$missing" -ne 0 ]; then
        exit 1
    fi
}

refresh_desktop_database() {
    if command -v update-desktop-database &>/dev/null; then
        update-desktop-database "$APP_DIR" 2>/dev/null || true
    fi
    if command -v gtk-update-icon-cache &>/dev/null; then
        gtk-update-icon-cache -f -t "$PREFIX/share/icons/hicolor" 2>/dev/null || true
    fi
}

# --- Uninstall 

do_uninstall() {
    echo "Uninstalling $APP_NAME from $PREFIX..."
    rm -fv "$BIN_DIR/$APP_NAME"
    rm -fv "$APP_DIR/$APP_NAME.desktop"
    rm -fv "$ICON_DIR/$APP_NAME.png"
    refresh_desktop_database
    echo "Done."
    exit 0
}

# --- Install

do_install() {
    validate_source_files

    echo "Installing $APP_NAME to $PREFIX..."

    # Binary
    install -Dm755 "$BASEDIR/$APP_NAME" "$BIN_DIR/$APP_NAME"
    echo "  -> $BIN_DIR/$APP_NAME"

    # Desktop entry
    install -Dm644 "$BASEDIR/desktop-assets/$APP_NAME.desktop" "$APP_DIR/$APP_NAME.desktop"
    echo "  -> $APP_DIR/$APP_NAME.desktop"

    # Icon
    install -Dm644 "$BASEDIR/appicon.png" "$ICON_DIR/$APP_NAME.png"
    echo "  -> $ICON_DIR/$APP_NAME.png"

    refresh_desktop_database

    echo "Done. You can run '$APP_NAME' from your terminal or application launcher."
}

# --- Main ---

case "${1:-}" in
    --help|-h)
        usage
        ;;
    --uninstall)
        check_dependencies
        check_root "$@"
        do_uninstall
        ;;
    "")
        check_dependencies
        check_runtime_libs
        check_root "$@"
        do_install
        ;;
    *)
        echo "Unknown option: $1" >&2
        usage
        ;;
esac
