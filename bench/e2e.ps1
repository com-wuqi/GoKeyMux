#Requires -Version 7.0
<#
.SYNOPSIS
    End-to-end gRPC load test for GoKeyMux using the noop backend.

.DESCRIPTION
    Builds GoKeyMux.exe and loadtest.exe into a timestamped temp directory,
    starts the server with a generated noop config, and runs cmd/loadtest over
    loopback TCP for a matrix of modes, concurrency levels and simulated
    backend latencies. Results are written to results.csv in the run directory.

    The noop backend performs no input injection, so this script is safe to run
    while using the machine. It measures the gRPC plus engine path, not driver
    latency.

.PARAMETER RepoRoot
    Repository root. Defaults to the parent directory of this script.

.PARAMETER Port
    Loopback TCP port used by the benchmark server.

.PARAMETER Duration
    Go duration passed to loadtest -d (per worker), for example 20s.

.PARAMETER Concurrency
    Worker counts to test.

.PARAMETER NoopLatencyMicros
    Simulated per-event backend latency values. The server is restarted for
    each value.

.PARAMETER OutputDir
    Parent directory for the run artifacts. Defaults to %TEMP%\gokeymux-bench.

.EXAMPLE
    pwsh ./bench/e2e.ps1 -Port 50071 -Duration 15s

.EXAMPLE
    pwsh ./bench/e2e.ps1 -Concurrency 1,8,32 -NoopLatencyMicros 0,100
#>
[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [ValidateRange(1, 65535)][int]$Port = 50071,
    [string]$Duration = "20s",
    [string[]]$Concurrency = @("1", "4", "16", "64"),
    [string[]]$NoopLatencyMicros = @("0", "50"),
    [string]$OutputDir = (Join-Path $env:TEMP "gokeymux-bench")
)

$ErrorActionPreference = "Stop"

# Native tool output (loadtest) contains microsecond values written as UTF-8.
# Decode it correctly so the CSV keeps readable units.
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)

function ConvertTo-IntArray {
    param([string[]]$Values)

    $result = [System.Collections.Generic.List[int]]::new()
    foreach ($value in $Values) {
        foreach ($part in ($value -split "[,\s]+")) {
            if ([string]::IsNullOrWhiteSpace($part)) { continue }
            $result.Add([int]$part)
        }
    }
    return $result.ToArray()
}

$concurrencyValues = ConvertTo-IntArray $Concurrency
$latencyValues = ConvertTo-IntArray $NoopLatencyMicros
if ($concurrencyValues.Count -eq 0) { throw "Concurrency must not be empty" }
if ($latencyValues.Count -eq 0) { throw "NoopLatencyMicros must not be empty" }

if ([string]::IsNullOrWhiteSpace($RepoRoot)) {
    $RepoRoot = Split-Path -Parent $PSScriptRoot
}
$RepoRoot = (Resolve-Path $RepoRoot).Path

$runDir = Join-Path $OutputDir ("run-" + (Get-Date -Format "yyyyMMdd-HHmmss"))
New-Item -ItemType Directory -Path $runDir -Force | Out-Null

$serverExe = Join-Path $runDir "GoKeyMux.exe"
$loadtestExe = Join-Path $runDir "loadtest.exe"
$configPath = Join-Path $runDir "config.json"

function Write-NoopConfig {
    param([int]$LatencyMicros)

    $config = [ordered]@{
        driveName                           = "noop"
        gRPCAddress                         = "127.0.0.1:$Port"
        logLevel                            = "error"
        gRPCKeepaliveTime                   = 2
        gRPCKeepaliveTimeOut                = 1
        gRPCKeepaliveMaxConnectionIdle      = 2
        gRPCEnforcementPolicyMinTime        = 10
        gRPCEnforcementPolicyPermitWithoutStream = $true
        gRPCServerShutdownTimeout           = 5
        winputWindowProcessName             = "Game.exe"
        winputWindowUseStaticIndex          = $true
        winputWindowIndex                   = 0
        pprofAddress                        = ""
        noopLatencyMicros                   = $LatencyMicros
        dispatchQueueSize                   = 1024
    }
    $config | ConvertTo-Json | Set-Content -Path $configPath -Encoding utf8
}

function Wait-LoopbackPort {
    param([int]$TimeoutSeconds = 15)

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $client = [System.Net.Sockets.TcpClient]::new()
        try {
            $client.Connect("127.0.0.1", $Port)
            return $true
        }
        catch {
            Start-Sleep -Milliseconds 100
        }
        finally {
            $client.Dispose()
        }
    }
    return $false
}

function Assert-LoopbackPortFree {
    $listener = $null
    try {
        $listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, $Port)
        $listener.Start()
    }
    catch {
        throw "port $Port is already in use; pass -Port with a free port"
    }
    finally {
        if ($null -ne $listener) { $listener.Stop() }
    }
}

function ConvertTo-ResultRow {
    param(
        [string]$Output,
        [string]$Mode,
        [int]$Concurrency,
        [int]$LatencyMicros
    )

    $line = $Output -split "`r?`n" |
        Where-Object { $_ -match "^$([regex]::Escape($Mode)):" } |
        Select-Object -First 1
    if (-not $line) {
        return $null
    }

    # Keep a fixed column set for every row: Export-Csv and Format-Table derive
    # their columns from the first object, and stream/unary report different
    # metrics.
    $row = [ordered]@{
        latency_us  = $LatencyMicros
        mode        = $Mode
        concurrency = $Concurrency
        total       = $null
        elapsed     = $null
        tps         = $null
        rps         = $null
        avg         = $null
        p50         = $null
        p90         = $null
        p95         = $null
        p99         = $null
        p999        = $null
        max         = $null
        failed      = $null
    }
    foreach ($match in [regex]::Matches($line, '([A-Za-z0-9_]+)=([^\s]+)')) {
        $row[$match.Groups[1].Value] = $match.Groups[2].Value
    }
    return [pscustomobject]$row
}

Write-Host "run directory: $runDir"
Write-Host "building binaries..."

Push-Location $RepoRoot
try {
    & go build -o $serverExe .
    if ($LASTEXITCODE -ne 0) { throw "go build (server) failed" }
    & go build -o $loadtestExe ./cmd/loadtest
    if ($LASTEXITCODE -ne 0) { throw "go build (loadtest) failed" }
}
finally {
    Pop-Location
}

$results = [System.Collections.Generic.List[object]]::new()
$server = $null

Assert-LoopbackPortFree

try {
    foreach ($latency in $latencyValues) {
        Write-NoopConfig -LatencyMicros $latency

        $stdout = Join-Path $runDir "server-$latency.out.log"
        $stderr = Join-Path $runDir "server-$latency.err.log"
        $server = Start-Process -FilePath $serverExe -WorkingDirectory $runDir -PassThru `
            -RedirectStandardOutput $stdout -RedirectStandardError $stderr

        try {
            if (-not (Wait-LoopbackPort)) {
                throw "server did not listen on 127.0.0.1:$Port (see $stderr)"
            }
            if ($server.HasExited) {
                throw "server exited during startup (see $stderr)"
            }
            Write-Host "server ready (noopLatencyMicros=$latency)"

            foreach ($mode in @("stream", "unary")) {
                foreach ($workers in $concurrencyValues) {
                    $label = "latency=${latency}us mode=$mode c=$workers"
                    Write-Host "running $label"
                    $output = (& $loadtestExe -addr "127.0.0.1:$Port" -mode $mode `
                        -c $workers -d $Duration -warmup 2s -key space 2>&1) -join "`n"
                    $row = ConvertTo-ResultRow -Output $output -Mode $mode `
                        -Concurrency $workers -LatencyMicros $latency
                    if ($null -eq $row) {
                        Write-Warning "no result line for $label"
                        Write-Host $output
                        continue
                    }
                    $results.Add($row)
                }
            }
        }
        finally {
            if ($null -ne $server -and -not $server.HasExited) {
                Stop-Process -Id $server.Id -Force
                Wait-Process -Id $server.Id -ErrorAction SilentlyContinue
            }
            $server = $null
        }
    }
}
finally {
    if ($null -ne $server -and -not $server.HasExited) {
        Stop-Process -Id $server.Id -Force
    }
}

$csvPath = Join-Path $runDir "results.csv"
$results | Export-Csv -Path $csvPath -NoTypeInformation -Encoding utf8

Write-Host ""
Write-Host "results: $csvPath"
$results | Select-Object latency_us, mode, concurrency, total, elapsed, tps, rps, p50, p99, max, failed |
    Format-Table -AutoSize | Out-String -Width 240 | Write-Host

if ($results.Count -eq 0) {
    throw "no benchmark results were collected"
}
