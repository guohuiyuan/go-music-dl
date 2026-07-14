[CmdletBinding()]
param(
    [string]$AssetsRoot = "desktop_app\ffmpeg\android",
    [string]$AndroidHome = $env:ANDROID_HOME,
    [Parameter(Mandatory = $true)]
    [string]$KeystoreFile,
    [Parameter(Mandatory = $true)]
    [string]$KeystorePassword
)

$ErrorActionPreference = "Stop"

function Get-LatestBuildTools {
    param([string]$SdkHome)

    if ([string]::IsNullOrWhiteSpace($SdkHome)) {
        throw "ANDROID_HOME is empty"
    }

    $buildToolsRoot = Join-Path $SdkHome "build-tools"
    $buildTools = Get-ChildItem -LiteralPath $buildToolsRoot -Directory |
        Sort-Object -Property @{ Expression = { try { [version]$_.Name } catch { [version]"0.0.0" } }; Descending = $true } |
        Select-Object -First 1

    if ($null -eq $buildTools) {
        throw "No Android build-tools found under $buildToolsRoot"
    }
    return $buildTools.FullName
}

function Remove-ApkEntries {
    param(
        [string]$ApkPath,
        [string]$Aapt
    )

    $entries = @(& $Aapt list $ApkPath)
    if ($LASTEXITCODE -ne 0) {
        throw "aapt list failed for $ApkPath"
    }

    $removeEntries = @(
        $entries | Where-Object {
            $_ -like "META-INF/*" -or
            $_ -like "assets/ffmpeg/*" -or
            $_ -match "^lib/[^/]+/lib(ffmpeg|ffprobe|c\+\+_shared)\.so$"
        }
    )

    if ($removeEntries.Count -eq 0) {
        return
    }

    & $Aapt remove $ApkPath @removeEntries
    if ($LASTEXITCODE -ne 0) {
        throw "aapt remove failed for $ApkPath"
    }
}

function Add-ApkAssets {
    param(
        [string]$ApkPath,
        [string[]]$Abis,
        [string]$AssetsRootFull,
        [string]$Aapt,
        [string]$WorkRoot
    )

    $stageRoot = Join-Path $WorkRoot "aapt-assets"
    $relativePaths = New-Object System.Collections.Generic.List[string]

    # Android 10+ (targetSdk >= 29) forbids executing binaries from the app's
    # private files dir. The only directory the platform marks executable is the
    # extracted native library dir, so ship ffmpeg/ffprobe as lib/<abi>/lib*.so.
    # PackageManager extracts these to nativeLibraryDir at install time (the APK
    # manifest keeps the default android:extractNativeLibs="true").
    $toolMap = @{
        "ffmpeg"           = "libffmpeg.so"
        "ffprobe"          = "libffprobe.so"
        "libc++_shared.so" = "libc++_shared.so"
    }

    foreach ($abi in $Abis) {
        foreach ($tool in @("ffmpeg", "ffprobe", "libc++_shared.so")) {
            $source = Join-Path $AssetsRootFull (Join-Path $abi $tool)
            if (-not (Test-Path $source)) {
                throw "Missing bundled $tool for $abi at $source"
            }

            $libName = $toolMap[$tool]
            $relativePath = "lib/$abi/$libName"
            $destination = Join-Path $stageRoot (Join-Path "lib\$abi" $libName)
            New-Item -ItemType Directory -Path ([IO.Path]::GetDirectoryName($destination)) -Force | Out-Null
            Copy-Item -LiteralPath $source -Destination $destination -Force
            $relativePaths.Add($relativePath)
        }
    }

    if ($relativePaths.Count -eq 0) {
        return
    }

    Push-Location $stageRoot
    try {
        $relativePathArray = @($relativePaths.ToArray())
        & $Aapt add $ApkPath @relativePathArray
        if ($LASTEXITCODE -ne 0) {
            throw "aapt add failed for $ApkPath"
        }
    }
    finally {
        Pop-Location
    }
}

function Inject-Tools {
    param(
        [string]$ApkPath,
        [string[]]$Abis,
        [string]$AssetsRootFull,
        [string]$Aapt,
        [string]$ZipAlign,
        [string]$ApkSigner,
        [string]$KeystoreFileFull,
        [string]$Password
    )

    if (-not (Test-Path $ApkPath)) {
        Write-Host "Skipping missing APK $ApkPath"
        return
    }

    $apkFull = [IO.Path]::GetFullPath($ApkPath)
    $workRoot = Join-Path ([IO.Path]::GetTempPath()) ("music-dl-apk-inject-" + [Guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Path $workRoot -Force | Out-Null

    try {
        $unsignedApk = Join-Path $workRoot ([IO.Path]::GetFileName($apkFull))
        $alignedApk = Join-Path $workRoot ("aligned-" + [IO.Path]::GetFileName($apkFull))
        Copy-Item -LiteralPath $apkFull -Destination $unsignedApk -Force

        Remove-ApkEntries -ApkPath $unsignedApk -Aapt $Aapt
        Add-ApkAssets -ApkPath $unsignedApk -Abis $Abis -AssetsRootFull $AssetsRootFull -Aapt $Aapt -WorkRoot $workRoot

        & $ZipAlign -f -p 4 $unsignedApk $alignedApk
        if ($LASTEXITCODE -ne 0) {
            throw "zipalign failed for $apkFull"
        }

        & $ApkSigner sign --ks-pass "pass:$Password" --ks $KeystoreFileFull $alignedApk
        if ($LASTEXITCODE -ne 0) {
            throw "apksigner sign failed for $apkFull"
        }

        & $ApkSigner verify --verbose $alignedApk
        if ($LASTEXITCODE -ne 0) {
            throw "apksigner verify failed for $apkFull"
        }

        Copy-Item -LiteralPath $alignedApk -Destination $apkFull -Force
        Write-Host "Bundled ffmpeg and ffprobe assets into $apkFull"
    }
    finally {
        Remove-Item -LiteralPath $workRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
}

$assetsRootFull = [IO.Path]::GetFullPath($AssetsRoot)
$keystoreFileFull = [IO.Path]::GetFullPath($KeystoreFile)
$buildTools = Get-LatestBuildTools -SdkHome $AndroidHome
$aapt = Join-Path $buildTools "aapt.exe"
$zipAlign = Join-Path $buildTools "zipalign.exe"
$apkSigner = Join-Path $buildTools "apksigner.bat"

if (-not (Test-Path $aapt)) {
    throw "aapt not found at $aapt"
}
if (-not (Test-Path $zipAlign)) {
    throw "zipalign not found at $zipAlign"
}
if (-not (Test-Path $apkSigner)) {
    throw "apksigner not found at $apkSigner"
}

$apkSpecs = @(
    @{ Path = "music-dl.apk";           Abis = @("armeabi-v7a", "arm64-v8a", "x86", "x86_64") },
    @{ Path = "music-dl_arm64-v8a.apk"; Abis = @("arm64-v8a") },
    @{ Path = "music-dl_x86_64.apk";    Abis = @("x86_64") }
)

foreach ($spec in $apkSpecs) {
    Inject-Tools `
        -ApkPath $spec.Path `
        -Abis $spec.Abis `
        -AssetsRootFull $assetsRootFull `
        -Aapt $aapt `
        -ZipAlign $zipAlign `
        -ApkSigner $apkSigner `
        -KeystoreFileFull $keystoreFileFull `
        -Password $KeystorePassword
}
