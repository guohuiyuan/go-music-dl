#!/usr/bin/env bash
set -euo pipefail

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "ERROR: iOS packaging must run on macOS with Xcode installed."
  exit 1
fi

DEFAULT_VERSION="1.0.0.1"
VERSION="${IOS_VERSION:-$DEFAULT_VERSION}"
COMMIT_COUNT="$(git rev-list --count HEAD 2>/dev/null || true)"
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

go install github.com/lianhong2758/gio-cmd/gogio@latest
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
  plutil -convert binary1 "$plist"

  local ats_value
  ats_value="$(plutil -extract NSAppTransportSecurity.NSAllowsLocalNetworking raw -o - "$plist")"
  if [[ "$ats_value" != "1" && "$ats_value" != "true" ]]; then
    echo "ERROR: failed to set NSAllowsLocalNetworking in Info.plist."
    exit 1
  fi
}

verify_ipa() {
  local ipa="$1"
  unzip -l "$ipa" | grep -E 'Payload/[^/]+\.app/Info\.plist' >/dev/null
  unzip -l "$ipa" | grep -E 'Payload/[^/]+\.app/MusicDL$' >/dev/null
}

build_unsigned_package() {
  local output_abs="$PWD/$IOS_UNSIGNED_OUTPUT"
  local empty_home="$TMP_ROOT/empty-home"
  local gogio_log="$TMP_ROOT/gogio-unsigned.log"
  local workdir
  local app_dir

  echo "iOS unsigned output: $IOS_UNSIGNED_OUTPUT"
  rm -f "$output_abs"
  mkdir -p "$empty_home"

  pushd desktop_app >/dev/null
  set +e
  HOME="$empty_home" GOPATH="$GO_GOPATH" GOMODCACHE="$GO_GOMODCACHE" GOCACHE="$GO_GOCACHE" gogio -target ios \
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
  (cd "$workdir" && zip -qr "$output_abs" Payload)
  verify_ipa "$output_abs"

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

  local output_abs="$PWD/$IOS_OUTPUT"
  local ipa_dir="$TMP_ROOT/signed-ipa"
  local app_dir
  local plist
  local sign_identity
  local provision_plist="$TMP_ROOT/provision.plist"
  local entitlements_plist="$TMP_ROOT/entitlements.plist"

  echo "iOS signed output: $IOS_OUTPUT"
  rm -f "$output_abs"

  pushd desktop_app >/dev/null
  gogio -target ios \
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

  rm -f "$output_abs"
  (cd "$ipa_dir" && zip -qr "$output_abs" Payload)
  verify_ipa "$output_abs"

  echo "Built signed iOS IPA: $IOS_OUTPUT"
}

if [[ "$IOS_UNSIGNED_ONLY" == "1" || "$IOS_UNSIGNED_ONLY" == "true" ]]; then
  build_unsigned_package
else
  build_signed_ipa
fi
