$ErrorActionPreference = 'Stop'

$profile = if ($env:COVERAGE_PROFILE) {
    $env:COVERAGE_PROFILE
} else {
    Join-Path ([System.IO.Path]::GetTempPath()) ("lql-client-go-coverage-{0}.out" -f [guid]::NewGuid())
}

try {
    & go test -covermode=atomic "-coverprofile=$profile" ./...
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }

    $summary = & go tool cover "-func=$profile" | Select-String '^total:' | Select-Object -Last 1
    if (-not $summary) {
        throw 'coverage summary was not produced'
    }
    if ($summary.Line -notmatch '([0-9]+(?:\.[0-9]+)?)%') {
        throw "could not parse coverage summary: $($summary.Line)"
    }
    $coverage = [double]$Matches[1]
    if ($coverage -lt 95) {
        throw "coverage $coverage% is below required 95%"
    }
    Write-Output ("coverage {0:N1}% (required >=95%)" -f $coverage)
} finally {
    if (-not $env:COVERAGE_PROFILE -and (Test-Path -LiteralPath $profile)) {
        Remove-Item -LiteralPath $profile -Force
    }
}
