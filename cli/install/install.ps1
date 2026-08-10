$ErrorActionPreference = "Stop"

$Repo = if ($env:LOZA_REPO) { $env:LOZA_REPO } else { "astraive/loza" }
$InstallDir = if ($env:LOZA_INSTALL_DIR) { $env:LOZA_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "Programs\loza\bin" }
$TempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("loza-install-" + [System.Guid]::NewGuid().ToString("N"))

New-Item -ItemType Directory -Force -Path $TempDir | Out-Null

try {
    $arch = switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture) {
        "X64" { "amd64" }
        "Arm64" { "arm64" }
        default { throw "Unsupported architecture: $($_)" }
    }

    $releases = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases" -Headers @{ "User-Agent" = "loza-installer" }
    $cliRelease = $releases | Where-Object { $_.tag_name -like "cli/v*" } | Select-Object -First 1
    if (-not $cliRelease) {
        throw "Could not find a LOZA CLI release."
    }

    $asset = $cliRelease.assets | Where-Object { $_.name -eq "loza_$($cliRelease.tag_name.TrimStart('cli/v'))_windows_$arch.zip" } | Select-Object -First 1
    if (-not $asset) {
        $asset = $cliRelease.assets | Where-Object { $_.name -like "loza_*_windows_$arch.zip" } | Select-Object -First 1
    }
    if (-not $asset) {
        throw "Could not find a LOZA CLI Windows $arch asset."
    }

    $zip = Join-Path $TempDir "loza.zip"
    Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $zip
    Expand-Archive -Path $zip -DestinationPath $TempDir -Force

    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    Copy-Item -Force -Path (Join-Path $TempDir "loza.exe") -Destination (Join-Path $InstallDir "loza.exe")

    Write-Host "Installed loza to $(Join-Path $InstallDir 'loza.exe')"
    if (($env:PATH -split ";") -notcontains $InstallDir) {
        Write-Host "Add $InstallDir to PATH to run loza from any PowerShell session."
    }
}
finally {
    Remove-Item -Recurse -Force -Path $TempDir -ErrorAction SilentlyContinue
}
