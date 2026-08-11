#Get server and key
param($server, $key, $tls)

# Agent repo can be overridden via environment variable SANTAIZI_AGENT_REPO
$agentrepo = if ($env:SANTAIZI_AGENT_REPO) { $env:SANTAIZI_AGENT_REPO } else { "hi2shark/santaizi-agent" }

# Download latest release from github
if($PSVersionTable.PSVersion.Major -lt 5){
    Write-Host "Require PS >= 5,your PSVersion:"$PSVersionTable.PSVersion.Major -BackgroundColor DarkGreen -ForegroundColor White
    Write-Host "Refer to the community article and install manually! https://nyko.me/2020/12/13/santaizi-windows-client.html" -BackgroundColor DarkRed -ForegroundColor Green
    exit
}

#  x86 or x64 or arm64
if ([System.Environment]::Is64BitOperatingSystem) {
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
        $file = "santaizi-agent_windows_arm64.zip"
    } else {
        $file = "santaizi-agent_windows_amd64.zip"
    }
}
else {
    $file = "santaizi-agent_windows_386.zip"
}

$agentreleases = "https://api.github.com/repos/$agentrepo/releases"

#重复运行自动更新
if (Test-Path "C:\santaizi\santaizi-agent.exe") {
    Write-Host "Santaizi monitoring already exists, delete and reinstall" -BackgroundColor DarkGreen -ForegroundColor White
    C:\santaizi\santaizi-agent.exe service uninstall
    Remove-Item "C:\santaizi" -Recurse
}

#TLS/SSL
Write-Host "Determining latest santaizi release" -BackgroundColor DarkGreen -ForegroundColor White
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
$agenttag = (Invoke-WebRequest -Uri $agentreleases -UseBasicParsing | ConvertFrom-Json)[0].tag_name

if ([string]::IsNullOrWhiteSpace($agenttag)) {
    $optionUrl = "https://fastly.jsdelivr.net/gh/$agentrepo/"
    Try {
        $response = Invoke-WebRequest -Uri $optionUrl -UseBasicParsing -TimeoutSec 10
        if ($response.StatusCode -eq 200) {
            $versiontext = $response.Content | findstr /c:"option.value"
            $version = [regex]::Match($versiontext, "@(\d+\.\d+\.\d+)").Groups[1].Value
            $agenttag = "v" + $version
        }
    } Catch {
        $optionUrl = "https://gcore.jsdelivr.net/gh/$agentrepo/"
        $response = Invoke-WebRequest -Uri $optionUrl -UseBasicParsing -TimeoutSec 10
        if ($response.StatusCode -eq 200) {
            $versiontext = $response.Content | findstr /c:"option.value"
            $version = [regex]::Match($versiontext, "@(\d+\.\d+\.\d+)").Groups[1].Value
            $agenttag = "v" + $version
        }
    }
}

#Region判断
$ipapi = ""
$region = "Unknown"
foreach ($url in ("https://dash.cloudflare.com/cdn-cgi/trace","https://developers.cloudflare.com/cdn-cgi/trace","https://1.0.0.1/cdn-cgi/trace")) {
    try {
        $ipapi = Invoke-RestMethod -Uri $url -TimeoutSec 5 -UseBasicParsing
        if ($ipapi -match "loc=(\w+)" ) {
            $region = $Matches[1]
            break
        }
    }
    catch {
        Write-Host "Error occurred while querying $url : $_"
    }
}

echo $ipapi

if($region -ne "CN"){
    $download = "https://github.com/$agentrepo/releases/download/$agenttag/$file"
    Write-Host "Location:$region,connect directly!" -BackgroundColor DarkRed -ForegroundColor Green
} else {
    # For CN users, try the gitee mirror if the official santaizihq agent repo is used
    $giteeRepo = $agentrepo -replace "^santaizihq/", "naibahq/"
    $download = "https://gitee.com/$giteeRepo/releases/download/$agenttag/$file"
    Write-Host "Location:CN,use mirror address" -BackgroundColor DarkRed -ForegroundColor Green
}

echo $download
Invoke-WebRequest $download -OutFile "C:\santaizi.zip"

#解压
Expand-Archive "C:\santaizi.zip" -DestinationPath "C:\temp" -Force
if (!(Test-Path "C:\santaizi")) { New-Item -Path "C:\santaizi" -type directory }
#整理文件
Move-Item -Path "C:\temp\santaizi-agent.exe" -Destination "C:\santaizi\santaizi-agent.exe"
#清理垃圾
Remove-Item "C:\santaizi.zip"
Remove-Item "C:\temp" -Recurse
#安装部分
$installArgs = @('-s', $server, '-p', $key)
if ($tls) {
    $installArgs += $tls
}
if ($args) {
    $installArgs += $args
}
C:\santaizi\santaizi-agent.exe service install @installArgs

#enjoy
Write-Host "Enjoy It!" -BackgroundColor DarkGreen -ForegroundColor Red
