[CmdletBinding()]
param(
    [string]$OutputRoot = "desktop_app\ffmpeg\android",
    [string]$BaseUrl = "https://sourceforge.net/projects/xabe-ffmpeg.mirror/files/executables",
    [string]$AndroidHome = $env:ANDROID_HOME,
    [string]$AndroidNdkRoot = $env:ANDROID_NDK_ROOT
)

$ErrorActionPreference = "Stop"

$items = @(
    @{ Abi = "armeabi-v7a"; Zip = "ffmpeg-android-arm.zip";    Triple = "arm-linux-androideabi" },
    @{ Abi = "arm64-v8a";  Zip = "ffmpeg-android-arm64.zip";  Triple = "aarch64-linux-android" },
    @{ Abi = "x86";         Zip = "ffmpeg-android-x86.zip";    Triple = "i686-linux-android" },
    @{ Abi = "x86_64";      Zip = "ffmpeg-android-x86_64.zip"; Triple = "x86_64-linux-android" }
)

$outputRootFull = [IO.Path]::GetFullPath($OutputRoot)
New-Item -ItemType Directory -Path $outputRootFull -Force | Out-Null

function Get-LatestNdkRoot {
    param(
        [string]$SdkHome,
        [string]$ExplicitNdkRoot
    )

    if (-not [string]::IsNullOrWhiteSpace($ExplicitNdkRoot) -and (Test-Path $ExplicitNdkRoot)) {
        return [IO.Path]::GetFullPath($ExplicitNdkRoot)
    }

    if ([string]::IsNullOrWhiteSpace($SdkHome)) {
        throw "ANDROID_HOME is empty and ANDROID_NDK_ROOT was not provided"
    }

    $ndkBundle = Join-Path $SdkHome "ndk-bundle"
    if (Test-Path (Join-Path $ndkBundle "source.properties")) {
        return [IO.Path]::GetFullPath($ndkBundle)
    }

    $ndkRoot = Join-Path $SdkHome "ndk"
    $ndk = Get-ChildItem -LiteralPath $ndkRoot -Directory |
        Sort-Object -Property @{ Expression = { try { [version]$_.Name } catch { [version]"0.0.0" } }; Descending = $true } |
        Select-Object -First 1

    if ($null -eq $ndk) {
        throw "No Android NDK found under $ndkRoot"
    }
    return $ndk.FullName
}

$ndkRootFull = Get-LatestNdkRoot -SdkHome $AndroidHome -ExplicitNdkRoot $AndroidNdkRoot
$ndkLibRoot = Join-Path $ndkRootFull "toolchains\llvm\prebuilt"
$ndkHost = Get-ChildItem -LiteralPath $ndkLibRoot -Directory |
    Where-Object { Test-Path (Join-Path $_.FullName "sysroot\usr\lib") } |
    Select-Object -First 1
if ($null -eq $ndkHost) {
    throw "No LLVM prebuilt sysroot found under $ndkLibRoot"
}

$workRoot = Join-Path ([IO.Path]::GetTempPath()) ("music-dl-android-ffmpeg-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $workRoot -Force | Out-Null

try {
    foreach ($item in $items) {
        $zipPath = Join-Path $workRoot $item.Zip
        $extractRoot = Join-Path $workRoot $item.Abi
        $abiRoot = Join-Path $outputRootFull $item.Abi
        $url = "$BaseUrl/$($item.Zip)/download"

        Write-Host "Downloading $($item.Zip)"
        & curl.exe -L --fail --retry 5 --retry-delay 5 -o $zipPath $url
        if ($LASTEXITCODE -ne 0) {
            throw "curl failed for $url"
        }

        Expand-Archive -Path $zipPath -DestinationPath $extractRoot -Force
        New-Item -ItemType Directory -Path $abiRoot -Force | Out-Null

        foreach ($tool in @("ffmpeg", "ffprobe")) {
            $source = Join-Path $extractRoot $tool
            if (-not (Test-Path $source)) {
                throw "$($item.Zip) does not contain $tool"
            }

            $target = Join-Path $abiRoot $tool
            Copy-Item -LiteralPath $source -Destination $target -Force

            $length = (Get-Item -LiteralPath $target).Length
            if ($length -lt 1048576) {
                throw "$target is unexpectedly small ($length bytes)"
            }
        }

        $libcxxSource = Join-Path $ndkHost.FullName (Join-Path "sysroot\usr\lib\$($item.Triple)" "libc++_shared.so")
        if (-not (Test-Path $libcxxSource)) {
            throw "libc++_shared.so not found for $($item.Abi) at $libcxxSource"
        }
        $libcxxTarget = Join-Path $abiRoot "libc++_shared.so"
        Copy-Item -LiteralPath $libcxxSource -Destination $libcxxTarget -Force
    }
}
finally {
    Remove-Item -LiteralPath $workRoot -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host "Android ffmpeg binaries prepared at $outputRootFull"
