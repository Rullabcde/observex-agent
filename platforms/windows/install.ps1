param(
    [Parameter(Mandatory = $true)]
    [string]$ApiKey,
    [Parameter(Mandatory = $true)]
    [string]$ApiUrl,
    [string]$BinaryPath = ".\uptimeid-agent-windows-amd64.exe"
)

$agentDir = "C:\ProgramData\UptimeID\Agent"
$agentExe = Join-Path $agentDir "uptimeid-agent.exe"
$envFile = Join-Path $agentDir ".env"

Write-Host "Installing UptimeID Agent..." -ForegroundColor Green

# Create directory if not exists
if (-not (Test-Path $agentDir)) {
    New-Item -ItemType Directory -Path $agentDir -Force | Out-Null
    Write-Host "  Created $agentDir"
}

# Copy binary
if (Test-Path $BinaryPath) {
    Copy-Item -Path $BinaryPath -Destination $agentExe -Force
    Write-Host "  Copied binary to $agentExe"
} else {
    Write-Error "Binary not found at $BinaryPath"
    exit 1
}

# Create .env file
@"
API_KEY=$ApiKey
API_URL=$ApiUrl
SEND_INTERVAL_SECONDS=5
MAX_LOG_SIZE_BYTES=400000
HTTP_TIMEOUT_SECONDS=10
"@ | Out-File -FilePath $envFile -Encoding UTF8
Write-Host "  Created $envFile"

# Create Windows service
$serviceName = "UptimeIDAgent"
$existing = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
if ($existing) {
    Write-Host "  Stopping existing service..."
    Stop-Service -Name $serviceName -Force -ErrorAction SilentlyContinue
    sc.exe delete $serviceName | Out-Null
    Start-Sleep -Seconds 2
}

New-Service -Name $serviceName `
    -BinaryPathName "`"$agentExe`"" `
    -DisplayName "UptimeID Monitoring Agent" `
    -Description "Collects and sends system metrics to UptimeID" `
    -StartupType Automatic

Start-Sleep -Seconds 1
Start-Service -Name $serviceName

Write-Host ""
Write-Host "UptimeID Agent installed and started successfully!" -ForegroundColor Green
Write-Host "  Service: $serviceName"
Write-Host "  Binary:  $agentExe"
Write-Host "  Config:  $envFile"
