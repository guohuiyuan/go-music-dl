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

Add-Type -AssemblyName System.IO.Compression
Add-Type -AssemblyName System.IO.Compression.FileSystem

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

function Remove-EntryIfExists {
    param(
        [System.IO.Compression.ZipArchive]$Zip,
        [string]$Name
    )

    $entry = $Zip.GetEntry($Name)
    if ($null -ne $entry) {
        $entry.Delete()
    }
}

function Add-FileEntry {
    param(
        [System.IO.Compression.ZipArchive]$Zip,
        [string]$Source,
        [string]$EntryName
    )

    Remove-EntryIfExists -Zip $Zip -Name $EntryName
    $entry = $Zip.CreateEntry($EntryName, [System.IO.Compression.CompressionLevel]::Optimal)
    $inputStream = [IO.File]::OpenRead($Source)
    try {
        $outputStream = $entry.Open()
        try {
            $inputStream.CopyTo($outputStream)
        }
        finally {
            $outputStream.Dispose()
        }
    }
    finally {
        $inputStream.Dispose()
    }
}

function Inject-Tools {
    param(
        [string]$ApkPath,
        [string[]]$Abis,
        [string]$AssetsRootFull,
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

        $zip = [IO.Compression.ZipArchive]::new(
            [IO.File]::Open($unsignedApk, [IO.FileMode]::Open, [IO.FileAccess]::ReadWrite),
            [IO.Compression.ZipArchiveMode]::Update
        )
        try {
            @($zip.Entries | Where-Object { $_.FullName -like "META-INF/*" }) | ForEach-Object { $_.Delete() }

            foreach ($abi in $Abis) {
                foreach ($tool in @("ffmpeg", "ffprobe")) {
                    $source = Join-Path $AssetsRootFull (Join-Path $abi $tool)
                    if (-not (Test-Path $source)) {
                        throw "Missing bundled $tool for $abi at $source"
                    }
                    Remove-EntryIfExists -Zip $zip -Name "lib/$abi/lib$tool.so"
                    Add-FileEntry -Zip $zip -Source $source -EntryName "assets/ffmpeg/$abi/$tool"
                }
            }
        }
        finally {
            $zip.Dispose()
        }

        & $ZipAlign -f 4 $unsignedApk $alignedApk
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
$zipAlign = Join-Path $buildTools "zipalign.exe"
$apkSigner = Join-Path $buildTools "apksigner.bat"

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
        -ZipAlign $zipAlign `
        -ApkSigner $apkSigner `
        -KeystoreFileFull $keystoreFileFull `
        -Password $KeystorePassword
}
