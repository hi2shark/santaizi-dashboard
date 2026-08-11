param(
    [Parameter(Mandatory = $true, Position = 0)] [string] $Server,
    [Parameter(Mandatory = $true, Position = 1)] [ValidateRange(1, 65535)] [int] $Port,
    [Parameter(Mandatory = $true, Position = 2)] [string] $Key,
    [switch] $Tls,
    [switch] $InsecureTls,
    [switch] $CleanInstall,
    [switch] $ConfirmCleanInstall,
    [switch] $DisableCpu,
    [switch] $DisableMemory,
    [switch] $DisableDisk,
    [switch] $DisableNetwork,
    [switch] $DisableConnections,
    [switch] $DisableProcesses,
    [switch] $Temperature,
    [switch] $Gpu,
    [switch] $DisableHostInfo,
    [switch] $DisableIpReport,
    [switch] $DisableHttpProbe,
    [switch] $DisableIcmpProbe,
    [switch] $DisableTcpProbe,
    [switch] $DisableNat
)

$ErrorActionPreference = "Stop"
$AgentRepository = if ($env:SANTAIZI_AGENT_REPO) { $env:SANTAIZI_AGENT_REPO } else { "hi2shark/santaizi-agent" }
$InstallDirectory = "C:\santaizi"
$AgentBinary = "C:\santaizi\santaizi-agent.exe"
$ConfigurationDirectory = "C:\ProgramData\santaizi"
$ConfigurationPath = "C:\ProgramData\santaizi\agent.yaml"
$DataDirectory = "C:\ProgramData\santaizi-agent"

if ($PSVersionTable.PSVersion.Major -lt 5) {
    throw "Santaizi Agent requires PowerShell 5 or later."
}
if ($CleanInstall -and -not $ConfirmCleanInstall) {
    throw "Clean install removes the existing agent configuration, identity, and WAL. Add -ConfirmCleanInstall after confirming this action."
}

if ($CleanInstall) {
    Write-Host "Running the confirmed clean installation..."
    if (Test-Path $AgentBinary) {
        try { & $AgentBinary service uninstall | Out-Null } catch { }
    }
    Stop-Service -Name "santaizi-agent" -Force -ErrorAction SilentlyContinue
    if (Test-Path $InstallDirectory) { Remove-Item -LiteralPath $InstallDirectory -Recurse -Force }
    if (Test-Path $ConfigurationPath) { Remove-Item -LiteralPath $ConfigurationPath -Force }
    if (Test-Path $DataDirectory) { Remove-Item -LiteralPath $DataDirectory -Recurse -Force }
}

$Architecture = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } elseif ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
$Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$AgentRepository/releases/latest" -UseBasicParsing
if (-not $Release.tag_name) { throw "Unable to determine the latest Santaizi Agent release." }

$ArchiveName = "santaizi-agent_windows_$Architecture.zip"
$DownloadUrl = "https://github.com/$AgentRepository/releases/download/$($Release.tag_name)/$ArchiveName"
$TemporaryRoot = Join-Path $env:TEMP ("santaizi-agent-install-" + [Guid]::NewGuid().ToString("N"))
$ArchivePath = Join-Path $TemporaryRoot $ArchiveName
$ExtractPath = Join-Path $TemporaryRoot "extract"

New-Item -ItemType Directory -Path $TemporaryRoot, $ExtractPath -Force | Out-Null
try {
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $ArchivePath -UseBasicParsing
    Expand-Archive -LiteralPath $ArchivePath -DestinationPath $ExtractPath -Force
    New-Item -ItemType Directory -Path $InstallDirectory, $ConfigurationDirectory, $DataDirectory -Force | Out-Null
    Copy-Item -LiteralPath (Join-Path $ExtractPath "santaizi-agent.exe") -Destination $AgentBinary -Force
} finally {
    if (Test-Path $TemporaryRoot) { Remove-Item -LiteralPath $TemporaryRoot -Recurse -Force }
}

try { & $AgentBinary service uninstall | Out-Null } catch { }

$Endpoint = if ($Server.StartsWith("[") -or -not $Server.Contains(":")) {
    "{0}:{1}" -f $Server, $Port
} else {
    "[{0}]:{1}" -f $Server, $Port
}

$InstallArguments = @(
    "--config", $ConfigurationPath,
    "--data-dir", $DataDirectory,
    "-s", $Endpoint,
    "-p", $Key
)

$Switches = @(
    @{ Enabled = $Tls; Flag = "--tls" },
    @{ Enabled = $InsecureTls; Flag = "--insecure" },
    @{ Enabled = $DisableCpu; Flag = "--disable-cpu" },
    @{ Enabled = $DisableMemory; Flag = "--disable-memory" },
    @{ Enabled = $DisableDisk; Flag = "--disable-disk" },
    @{ Enabled = $DisableNetwork; Flag = "--disable-network" },
    @{ Enabled = $DisableConnections; Flag = "--disable-connections" },
    @{ Enabled = $DisableProcesses; Flag = "--disable-processes" },
    @{ Enabled = $Temperature; Flag = "--temperature" },
    @{ Enabled = $Gpu; Flag = "--gpu" },
    @{ Enabled = $DisableHostInfo; Flag = "--disable-host-info" },
    @{ Enabled = $DisableIpReport; Flag = "--disable-ip-report" },
    @{ Enabled = $DisableHttpProbe; Flag = "--disable-http-probe" },
    @{ Enabled = $DisableIcmpProbe; Flag = "--disable-icmp-probe" },
    @{ Enabled = $DisableTcpProbe; Flag = "--disable-tcp-probe" },
    @{ Enabled = $DisableNat; Flag = "--disable-nat" }
)
foreach ($Item in $Switches) {
    if ($Item.Enabled) { $InstallArguments += $Item.Flag }
}

& $AgentBinary service install @InstallArguments
if ($LASTEXITCODE -ne 0) { throw "Santaizi Agent service installation failed." }

Write-Host "Santaizi Agent installed successfully."
