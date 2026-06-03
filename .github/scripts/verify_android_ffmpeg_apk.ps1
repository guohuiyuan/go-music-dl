[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$ApkPath,
    [Parameter(Mandatory = $true)]
    [string[]]$Abis,
    [string]$AndroidHome = $env:ANDROID_HOME
)

$ErrorActionPreference = "Stop"

Add-Type -AssemblyName System.IO.Compression
Add-Type -AssemblyName System.IO.Compression.FileSystem

if (-not (Test-Path $ApkPath)) {
    throw "APK not found: $ApkPath"
}

$apkFull = [IO.Path]::GetFullPath($ApkPath)
$zip = [IO.Compression.ZipFile]::OpenRead($apkFull)
try {
$expandedAbis = @($Abis | ForEach-Object { $_ -split "," } | ForEach-Object { $_.Trim() } | Where-Object { $_ })

foreach ($abi in $expandedAbis) {
        foreach ($tool in @("ffmpeg", "ffprobe")) {
            $legacyLibEntryName = "lib/$abi/lib$tool.so"
            if ($null -ne $zip.GetEntry($legacyLibEntryName)) {
                throw "$apkFull must not contain $legacyLibEntryName; bundled executables belong under assets/ffmpeg"
            }

            $entryName = "assets/ffmpeg/$abi/$tool"
            $entry = $zip.GetEntry($entryName)
            if ($null -eq $entry) {
                throw "$apkFull is missing $entryName"
            }
            if ($entry.Length -lt 1048576) {
                throw "$entryName in $apkFull is unexpectedly small ($($entry.Length) bytes)"
            }
        }
    }
}
finally {
    $zip.Dispose()
}

$buildToolsRoot = Join-Path $AndroidHome "build-tools"
$buildTools = Get-ChildItem -LiteralPath $buildToolsRoot -Directory |
    Sort-Object -Property @{ Expression = { try { [version]$_.Name } catch { [version]"0.0.0" } }; Descending = $true } |
    Select-Object -First 1
if ($null -eq $buildTools) {
    throw "No Android build-tools found under $buildToolsRoot"
}
$apkSigner = Join-Path $buildTools.FullName "apksigner.bat"
& $apkSigner verify --verbose $apkFull
if ($LASTEXITCODE -ne 0) {
    throw "apksigner verify failed for $apkFull"
}

Write-Host "Verified bundled ffmpeg and ffprobe in $apkFull"
