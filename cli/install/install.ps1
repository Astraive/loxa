$ErrorActionPreference = "Stop"

$Repo = if ($env:LOXA_REPO) { $env:LOXA_REPO } else { "astraive/loxa" }
$InstallDir = if ($env:LOXA_INSTALL_DIR) { $env:LOXA_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "Programs\loxa\bin" }
$TempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("loxa-install-" + [System.Guid]::NewGuid().ToString("N"))

New-Item -ItemType Directory -Force -Path $TempDir | Out-Null

try {
    $arch = switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture) {
        "X64" { "amd64" }
        "Arm64" { "arm64" }
        default { throw "Unsupported architecture: $($_)" }
    }

    $releases = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases" -Headers @{ "User-Agent" = "loxa-installer" }
    $cliRelease = $releases | Where-Object { $_.tag_name -like "cli/v*" } | Select-Object -First 1
    if (-not $cliRelease) {
        throw "Could not find a LOXA CLI release."
    }

    $asset = $cliRelease.assets | Where-Object { $_.name -eq "loxa_$($cliRelease.tag_name.TrimStart('cli/v'))_windows_$arch.zip" } | Select-Object -First 1
    if (-not $asset) {
        $asset = $cliRelease.assets | Where-Object { $_.name -like "loxa_*_windows_$arch.zip" } | Select-Object -First 1
    }
    if (-not $asset) {
        throw "Could not find a LOXA CLI Windows $arch asset."
    }

    $zip = Join-Path $TempDir "loxa.zip"
    Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $zip
    Expand-Archive -Path $zip -DestinationPath $TempDir -Force

    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    Copy-Item -Force -Path (Join-Path $TempDir "loxa.exe") -Destination (Join-Path $InstallDir "loxa.exe")

    Write-Host "Installed loxa to $(Join-Path $InstallDir 'loxa.exe')"
    if (($env:PATH -split ";") -notcontains $InstallDir) {
        Write-Host "Add $InstallDir to PATH to run loxa from any PowerShell session."
    }
}
finally {
    Remove-Item -Recurse -Force -Path $TempDir -ErrorAction SilentlyContinue
}
