# Build GoKeyMux with version info injected via -ldflags.
# Usage: .\build.ps1
$Version = git describe --tags --always --dirty 2>$null
if ($LASTEXITCODE -ne 0 -or -not $Version) { $Version = 'dev' }

$Commit = git rev-parse --short HEAD 2>$null
if ($LASTEXITCODE -ne 0 -or -not $Commit) { $Commit = 'unknown' }

$BuildTime = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')

$ServerLdFlags = "-s -w -X GoKeyMux/version.Version=$Version -X GoKeyMux/version.Commit=$Commit -X GoKeyMux/version.BuildTime=$BuildTime"

go build -trimpath -ldflags $ServerLdFlags -o GoKeyMux.exe .
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

go build -trimpath -ldflags "-s -w" -o keycycle.exe ./cmd/keycycle
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

go build -trimpath -ldflags "-s -w" -o loadtest.exe ./cmd/loadtest
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "Built GoKeyMux $Version (commit=$Commit built=$BuildTime)"
