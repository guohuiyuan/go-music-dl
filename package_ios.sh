#!/usr/bin/env bash
set -euo pipefail

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "ERROR: iOS packaging must run on macOS with Xcode installed."
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEFAULT_VERSION="1.0.0.1"
VERSION="${IOS_VERSION:-$DEFAULT_VERSION}"
COMMIT_COUNT="$(git -C "$SCRIPT_DIR" rev-list --count HEAD 2>/dev/null || true)"
if [[ -n "$COMMIT_COUNT" && "${IOS_VERSION:-}" == "" ]]; then
  VERSION="1.0.0.${COMMIT_COUNT}"
fi

IOS_APP_ID="${IOS_APP_ID:-com.guohuiyuan.musicdl}"
IOS_OUTPUT="${IOS_OUTPUT:-music-dl-ios.ipa}"
IOS_UNSIGNED_OUTPUT="${IOS_UNSIGNED_OUTPUT:-music-dl-ios-unsigned.ipa}"
IOS_UNSIGNED_ONLY="${IOS_UNSIGNED_ONLY:-0}"
IOS_MIN_SDK="${IOS_MIN_SDK:-13}"
IOS_PROVISION_PROFILE="${IOS_PROVISION_PROFILE:-${GOGIO_SIGNKEY:-}}"

TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "$TMP_ROOT"' EXIT

echo "iOS app id: $IOS_APP_ID"
echo "iOS version: $VERSION"
echo "iOS minimum version: $IOS_MIN_SDK"

GOGIO_BIN="$TMP_ROOT/gogio"
(
  cd "$SCRIPT_DIR/tools/gio-cmd"
  GOWORK=off go build -trimpath -o "$GOGIO_BIN" ./gogio
)
GO_GOPATH="$(go env GOPATH)"
GO_GOMODCACHE="$(go env GOMODCACHE)"
GO_GOCACHE="$(go env GOCACHE)"

add_local_networking_exception() {
  local plist="$1/Info.plist"
  if [[ ! -f "$plist" ]]; then
    echo "ERROR: app bundle is missing Info.plist: $plist"
    exit 1
  fi

  /usr/libexec/PlistBuddy -c "Add :NSAppTransportSecurity dict" "$plist" >/dev/null 2>&1 || true
  /usr/libexec/PlistBuddy -c "Set :NSAppTransportSecurity:NSAllowsLocalNetworking true" "$plist" >/dev/null 2>&1 || \
    /usr/libexec/PlistBuddy -c "Add :NSAppTransportSecurity:NSAllowsLocalNetworking bool true" "$plist" >/dev/null
  /usr/libexec/PlistBuddy -c "Set :NSAppTransportSecurity:NSAllowsArbitraryLoadsInWebContent true" "$plist" >/dev/null 2>&1 || \
    /usr/libexec/PlistBuddy -c "Add :NSAppTransportSecurity:NSAllowsArbitraryLoadsInWebContent bool true" "$plist" >/dev/null
  /usr/libexec/PlistBuddy -c "Set :NSLocalNetworkUsageDescription MusicDL uses the local network to connect to its embedded web server." "$plist" >/dev/null 2>&1 || \
    /usr/libexec/PlistBuddy -c "Add :NSLocalNetworkUsageDescription string MusicDL uses the local network to connect to its embedded web server." "$plist" >/dev/null
  plutil -lint "$plist" >/dev/null
  plutil -convert binary1 "$plist"

  local local_networking
  local web_content
  local_networking="$(plutil -extract NSAppTransportSecurity.NSAllowsLocalNetworking raw -o - "$plist")"
  web_content="$(plutil -extract NSAppTransportSecurity.NSAllowsArbitraryLoadsInWebContent raw -o - "$plist")"
  if [[ "$local_networking" != "1" && "$local_networking" != "true" ]]; then
    echo "ERROR: failed to set NSAllowsLocalNetworking in Info.plist."
    exit 1
  fi
  if [[ "$web_content" != "1" && "$web_content" != "true" ]]; then
    echo "ERROR: failed to set NSAllowsArbitraryLoadsInWebContent in Info.plist."
    exit 1
  fi
}

verify_app_bundle() {
  local app_dir="$1"
  local plist="$app_dir/Info.plist"
  local executable_name
  local executable
  local bundle_type
  local minimum_os
  local archs
  local local_networking
  local web_content

  if [[ ! -d "$app_dir" || ! -f "$plist" ]]; then
    echo "ERROR: invalid iOS app bundle: $app_dir"
    exit 1
  fi
  plutil -lint "$plist" >/dev/null

  executable_name="$(plutil -extract CFBundleExecutable raw -o - "$plist")"
  if [[ -z "$executable_name" ]]; then
    echo "ERROR: Info.plist has no CFBundleExecutable."
    exit 1
  fi
  executable="$app_dir/$executable_name"
  if [[ ! -f "$executable" || ! -x "$executable" ]]; then
    echo "ERROR: iOS executable is missing or not executable: $executable"
    exit 1
  fi

  bundle_type="$(plutil -extract CFBundlePackageType raw -o - "$plist")"
  if [[ "$bundle_type" != "APPL" ]]; then
    echo "ERROR: CFBundlePackageType is $bundle_type, expected APPL."
    exit 1
  fi

  minimum_os="$(plutil -extract MinimumOSVersion raw -o - "$plist")"
  if [[ -z "$minimum_os" ]]; then
    echo "ERROR: Info.plist has no MinimumOSVersion."
    exit 1
  fi

  archs="$(lipo -archs "$executable")"
  if [[ " $archs " != *" arm64 "* ]]; then
    echo "ERROR: iOS executable does not contain arm64 (architectures: $archs)."
    exit 1
  fi

  local_networking="$(plutil -extract NSAppTransportSecurity.NSAllowsLocalNetworking raw -o - "$plist")"
  web_content="$(plutil -extract NSAppTransportSecurity.NSAllowsArbitraryLoadsInWebContent raw -o - "$plist")"
  if [[ "$local_networking" != "1" && "$local_networking" != "true" ]]; then
    echo "ERROR: NSAllowsLocalNetworking is not enabled in the app bundle."
    exit 1
  fi
  if [[ "$web_content" != "1" && "$web_content" != "true" ]]; then
    echo "ERROR: NSAllowsArbitraryLoadsInWebContent is not enabled in the app bundle."
    exit 1
  fi

  echo "Verified iOS bundle: executable=$executable_name archs=$archs minimumOS=$minimum_os"
}

verify_ipa() {
  local ipa="$1"
  unzip -tq "$ipa" >/dev/null
  unzip -l "$ipa" | grep -E 'Payload/[^/]+\.app/Info\.plist' >/dev/null
  unzip -l "$ipa" | grep -E 'Payload/[^/]+\.app/MusicDL$' >/dev/null
}

write_checksum() {
  local ipa_abs="$1"
  local ipa_dir
  local ipa_name
  ipa_dir="$(dirname "$ipa_abs")"
  ipa_name="$(basename "$ipa_abs")"
  (cd "$ipa_dir" && shasum -a 256 "$ipa_name" > "$ipa_name.sha256")
  echo "SHA256: $(cat "$ipa_abs.sha256")"
}

build_unsigned_package() {
  local output_abs="$SCRIPT_DIR/$IOS_UNSIGNED_OUTPUT"
  local empty_home="$TMP_ROOT/empty-home"
  local gogio_log="$TMP_ROOT/gogio-unsigned.log"
  local workdir
  local app_dir

  echo "iOS unsigned output: $IOS_UNSIGNED_OUTPUT"
  rm -f "$output_abs"
  mkdir -p "$empty_home"

  pushd "$SCRIPT_DIR/desktop_app" >/dev/null
  set +e
  HOME="$empty_home" GOPATH="$GO_GOPATH" GOMODCACHE="$GO_GOMODCACHE" GOCACHE="$GO_GOCACHE" "$GOGIO_BIN" -target ios \
    -buildmode exe \
    -work \
    -o "../$IOS_UNSIGNED_OUTPUT" \
    -appid "$IOS_APP_ID" \
    -name MusicDL \
    -version "$VERSION" \
    -minsdk "$IOS_MIN_SDK" \
    -icon ../winres/icon_256x256.png \
    github.com/guohuiyuan/go-music-dl/desktop_app 2>"$gogio_log"
  local gogio_status=$?
  set -e
  popd >/dev/null

  cat "$gogio_log"
  workdir="$(sed -n 's/^WORKDIR=//p' "$gogio_log" | tail -n 1)"
  app_dir="$(find "$workdir/Payload" -maxdepth 1 -type d -name '*.app' -print -quit 2>/dev/null || true)"
  if [[ -z "$workdir" || -z "$app_dir" ]]; then
    echo "ERROR: gogio did not leave a device app bundle for user signing. Exit status: $gogio_status"
    exit 1
  fi

  add_local_networking_exception "$app_dir"
  verify_app_bundle "$app_dir"
  (cd "$workdir" && zip -qr "$output_abs" Payload)
  verify_ipa "$output_abs"
  write_checksum "$output_abs"

  echo "Built unsigned iOS signing package: $IOS_UNSIGNED_OUTPUT"
  echo "This package is for re-signing only and cannot be installed directly."
}

build_signed_ipa() {
  if [[ -z "$IOS_PROVISION_PROFILE" ]]; then
    cat >&2 <<'EOF'
ERROR: signed iOS IPA builds require a provisioning profile.

Set IOS_PROVISION_PROFILE=/path/to/profile.mobileprovision before running this script.
Use IOS_UNSIGNED_ONLY=1 to produce an unsigned signing package for users to re-sign themselves.
EOF
    exit 1
  fi

  if [[ ! -f "$IOS_PROVISION_PROFILE" ]]; then
    echo "ERROR: IOS_PROVISION_PROFILE does not exist: $IOS_PROVISION_PROFILE"
    exit 1
  fi

  local output_abs="$SCRIPT_DIR/$IOS_OUTPUT"
  local ipa_dir="$TMP_ROOT/signed-ipa"
  local app_dir
  local plist
  local sign_identity
  local provision_plist="$TMP_ROOT/provision.plist"
  local entitlements_plist="$TMP_ROOT/entitlements.plist"

  echo "iOS signed output: $IOS_OUTPUT"
  rm -f "$output_abs"

  pushd "$SCRIPT_DIR/desktop_app" >/dev/null
  "$GOGIO_BIN" -target ios \
    -buildmode exe \
    -o "../$IOS_OUTPUT" \
    -appid "$IOS_APP_ID" \
    -name MusicDL \
    -version "$VERSION" \
    -minsdk "$IOS_MIN_SDK" \
    -icon ../winres/icon_256x256.png \
    -signkey "$IOS_PROVISION_PROFILE" \
    github.com/guohuiyuan/go-music-dl/desktop_app
  popd >/dev/null

  if [[ ! -f "$output_abs" ]]; then
    echo "ERROR: expected IPA was not generated: $IOS_OUTPUT"
    exit 1
  fi

  mkdir -p "$ipa_dir"
  unzip -q "$output_abs" -d "$ipa_dir"
  app_dir="$(find "$ipa_dir/Payload" -maxdepth 1 -type d -name '*.app' -print -quit)"
  if [[ -z "$app_dir" ]]; then
    echo "ERROR: IPA does not contain a Payload/*.app bundle."
    exit 1
  fi

  add_local_networking_exception "$app_dir"
  verify_app_bundle "$app_dir"

  sign_identity="${IOS_CODESIGN_IDENTITY:-}"
  if [[ -z "$sign_identity" ]]; then
    sign_identity="$(codesign -dv --verbose=4 "$app_dir" 2>&1 | sed -n 's/^Authority=//p' | head -n 1 || true)"
  fi
  if [[ -z "$sign_identity" ]]; then
    echo "ERROR: failed to detect iOS signing identity; set IOS_CODESIGN_IDENTITY explicitly."
    exit 1
  fi

  security cms -D -i "$app_dir/embedded.mobileprovision" -o "$provision_plist"
  /usr/libexec/PlistBuddy -x -c "Print:Entitlements" "$provision_plist" > "$entitlements_plist"

  codesign --force --deep --sign "$sign_identity" --entitlements "$entitlements_plist" "$app_dir"
  codesign --verify --deep --strict "$app_dir"
  verify_app_bundle "$app_dir"

  rm -f "$output_abs"
  (cd "$ipa_dir" && zip -qr "$output_abs" Payload)
  verify_ipa "$output_abs"
  write_checksum "$output_abs"

  echo "Built signed iOS IPA: $IOS_OUTPUT"
}

if [[ "$IOS_UNSIGNED_ONLY" == "1" || "$IOS_UNSIGNED_ONLY" == "true" ]]; then
  build_unsigned_package
else
  build_signed_ipa
fi
