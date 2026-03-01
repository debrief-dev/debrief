#!/bin/bash
# build_appstore.sh - Script to build and package Debrief for the Mac App Store
#
# Usage: ./build_appstore.sh <version> [build_number]
# Example: ./build_appstore.sh 0.1.0
#          ./build_appstore.sh v0.1.0 42

set -euo pipefail

# --- Cleanup on failure ---
cleanup() {
  echo "ERROR: Build failed. Cleaning up partial artifacts..."
  rm -rf ./dist/appstore
  rm -rf ./build/appstore
}
trap cleanup ERR

# --- Help / Usage ---
if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  echo "Usage: $0 <version> [build_number]"
  echo ""
  echo "  version       Semantic version (e.g., 0.1.0 or v0.1.0)"
  echo "  build_number  Optional monotonic build number (defaults to YYYYMMDDHHmmss)"
  echo ""
  echo "Required environment variables:"
  echo "  APPLE_TEAM_ID   Your Apple Developer Team ID"
  echo ""
  echo "Optional environment variables:"
  echo "  APPLE_ID                      Apple ID for upload"
  echo "  APPLE_APP_SPECIFIC_PASSWORD   App-specific password for upload"
  exit 0
fi

# --- Validate prerequisites ---
for cmd in gogio lipo codesign productbuild pkgutil plutil; do
  if ! command -v "$cmd" &>/dev/null; then
    echo "ERROR: Required tool '$cmd' is not installed or not in PATH"
    exit 1
  fi
done

if [ -z "${APPLE_TEAM_ID:-}" ]; then
  echo "ERROR: APPLE_TEAM_ID environment variable is not set"
  exit 1
fi

# --- Version information ---
VERSION=$(echo "${1:-}" | sed 's/^v//')
if [ -z "$VERSION" ]; then
  echo "ERROR: Version must be provided as the first argument (e.g., 0.1.0)"
  echo "Run '$0 --help' for usage information."
  exit 1
fi

# Use provided build number or generate one with seconds for uniqueness
BUILD_NUMBER="${2:-$(date +%Y%m%d%H%M%S)}"

# --- Configuration ---
APP_NAME="Debrief"
BUNDLE_ID="rest.debrief.app"
ICON_PNG="./assets/appicon.png"
ICON_ICNS="./assets/appicon.icns"
SIGN_APP="3rd Party Mac Developer Application: Yauheni Basiakou ($APPLE_TEAM_ID)"
SIGN_INSTALLER="3rd Party Mac Developer Installer: Yauheni Basiakou ($APPLE_TEAM_ID)"
MIN_MACOS_VERSION="11.0"

DIST_BASE="./dist/appstore"
UNIVERSAL_APP="$DIST_BASE/universal/$APP_NAME.app"
BUILD_DIR="./build/appstore"

echo "============================================="
echo "Building $APP_NAME v$VERSION (build $BUILD_NUMBER) for App Store"
echo "============================================="

# --- Validate required files ---
if [ ! -f "$ICON_PNG" ]; then
  echo "ERROR: App icon not found at $ICON_PNG"
  exit 1
fi

if [ ! -f "$ICON_ICNS" ]; then
  echo "ERROR: ICNS icon not found at $ICON_ICNS"
  exit 1
fi

# --- Create directories ---
mkdir -p "$DIST_BASE/amd64"
mkdir -p "$DIST_BASE/arm64"
mkdir -p "$UNIVERSAL_APP/Contents/MacOS"
mkdir -p "$UNIVERSAL_APP/Contents/Resources"
mkdir -p "$BUILD_DIR"

# --- Create Info.plist ---
cat > "$BUILD_DIR/Info.plist" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleDevelopmentRegion</key>
  <string>en</string>
  <key>CFBundleExecutable</key>
  <string>$APP_NAME</string>
  <key>CFBundleIdentifier</key>
  <string>$BUNDLE_ID</string>
  <key>CFBundleInfoDictionaryVersion</key>
  <string>6.0</string>
  <key>CFBundleName</key>
  <string>$APP_NAME</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleShortVersionString</key>
  <string>$VERSION</string>
  <key>CFBundleVersion</key>
  <string>$BUILD_NUMBER</string>
  <key>LSMinimumSystemVersion</key>
  <string>$MIN_MACOS_VERSION</string>
  <key>NSHighResolutionCapable</key>
  <true/>
  <key>NSPrincipalClass</key>
  <string>NSApplication</string>
  <key>LSApplicationCategoryType</key>
  <string>public.app-category.developer-tools</string>
  <key>CFBundleIconFile</key>
  <string>appicon</string>
</dict>
</plist>
EOF

# --- Create entitlements ---
cat > "$BUILD_DIR/entitlements.plist" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>com.apple.security.app-sandbox</key>
  <true/>
  <key>com.apple.security.network.client</key>
  <true/>
  <key>com.apple.security.cs.allow-jit</key>
  <true/>
</dict>
</plist>
EOF

# --- Build both architectures ---
echo ""
echo "[1/6] Building for Intel (amd64)..."
gogio \
  -ldflags="-X version.AppVersion=$VERSION" \
  -appid="$BUNDLE_ID" \
  -icon="$ICON_PNG" \
  -target=macos \
  -arch=amd64 \
  -o "$DIST_BASE/amd64/$APP_NAME.app" .

echo "[2/6] Building for Apple Silicon (arm64)..."
gogio \
  -ldflags="-X version.AppVersion=$VERSION" \
  -appid="$BUNDLE_ID" \
  -icon="$ICON_PNG" \
  -target=macos \
  -arch=arm64 \
  -o "$DIST_BASE/arm64/$APP_NAME.app" .

# --- Create universal app bundle ---
echo "[3/6] Creating universal app bundle..."

# Copy resources from arm64 build
if [ -d "$DIST_BASE/arm64/$APP_NAME.app/Contents/Resources" ] && \
   [ -n "$(ls -A "$DIST_BASE/arm64/$APP_NAME.app/Contents/Resources" 2>/dev/null)" ]; then
  cp -R "$DIST_BASE/arm64/$APP_NAME.app/Contents/Resources/"* "$UNIVERSAL_APP/Contents/Resources/"
fi

# Use our custom Info.plist directly
cp "$BUILD_DIR/Info.plist" "$UNIVERSAL_APP/Contents/"

# Copy the ICNS icon
cp "$ICON_ICNS" "$UNIVERSAL_APP/Contents/Resources/"

# Create universal binary with lipo
echo "[4/6] Creating universal binary..."
lipo -create \
  "$DIST_BASE/amd64/$APP_NAME.app/Contents/MacOS/$APP_NAME" \
  "$DIST_BASE/arm64/$APP_NAME.app/Contents/MacOS/$APP_NAME" \
  -output "$UNIVERSAL_APP/Contents/MacOS/$APP_NAME"

# --- Sign ---
echo "[5/6] Signing universal app..."
codesign --force \
  --options runtime \
  --entitlements "$BUILD_DIR/entitlements.plist" \
  --deep \
  --sign "$SIGN_APP" \
  "$UNIVERSAL_APP"

echo "Verifying code signature..."
codesign -dvv "$UNIVERSAL_APP"

# --- Package ---
echo "[6/6] Building App Store package..."
PKG_OUTPUT="./dist/$APP_NAME-v$VERSION.pkg"

productbuild \
  --component "$UNIVERSAL_APP" /Applications \
  --sign "$SIGN_INSTALLER" \
  "$PKG_OUTPUT"

echo "Verifying package signature..."
pkgutil --check-signature "$PKG_OUTPUT"

# --- Done ---
echo ""
echo "============================================="
echo "Build complete!"
echo "  Package: $PKG_OUTPUT"
echo "  Version: $VERSION (build $BUILD_NUMBER)"
echo "============================================="
echo ""
echo "To upload to App Store Connect, use Transporter.app or run:"
echo "  xcrun notarytool submit $PKG_OUTPUT \\"
echo "    --apple-id \"\$APPLE_ID\" \\"
echo "    --password \"\$APPLE_APP_SPECIFIC_PASSWORD\" \\"
echo "    --team-id \"$APPLE_TEAM_ID\" \\"
echo "    --wait"