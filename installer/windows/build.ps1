#Requires -Version 5.1
<#
.SYNOPSIS
  Build merchant one-click installer: OdooPrintAgentSetup.exe

.DESCRIPTION
  1) go build agent
  2) fetch SumatraPDF + nssm into dist/windows
  3) compile Inno Setup script
#>
$ErrorActionPreference = "Stop"
$Root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$Dist = Join-Path $Root "dist\windows"
New-Item -ItemType Directory -Force -Path $Dist | Out-Null

Write-Host "==> Building odoo-print-agent.exe"
Push-Location $Root
try {
  & go build -ldflags "-s -w -X main.Version=1.0.1" -o (Join-Path $Dist "odoo-print-agent.exe") .
  if ($LASTEXITCODE -ne 0) { throw "go build failed" }
} finally {
  Pop-Location
}

$sumatraZip = Join-Path $env:TEMP "SumatraPDF-3.6.1-64.zip"
$sumatraUrl = "https://www.sumatrapdfreader.org/dl/rel/3.6.1/SumatraPDF-3.6.1-64.zip"
Write-Host "==> Downloading SumatraPDF"
Invoke-WebRequest -Uri $sumatraUrl -OutFile $sumatraZip -UseBasicParsing
$sumatraExtract = Join-Path $env:TEMP "SumatraPDF-extract"
Remove-Item -Recurse -Force $sumatraExtract -ErrorAction SilentlyContinue
Expand-Archive -Path $sumatraZip -DestinationPath $sumatraExtract -Force
$sumatraExe = Get-ChildItem $sumatraExtract -Recurse -Filter "SumatraPDF.exe" | Select-Object -First 1
if (-not $sumatraExe) {
  # Official "zip" may contain a renamed portable EXE (e.g. SumatraPDF-3.6.1-64.exe)
  $sumatraExe = Get-ChildItem $sumatraExtract -Recurse -Filter "SumatraPDF*.exe" | Select-Object -First 1
}
if (-not $sumatraExe) {
  $local = Join-Path $env:LOCALAPPDATA "SumatraPDF\SumatraPDF.exe"
  if (Test-Path $local) { $sumatraExe = Get-Item $local }
}
if (-not $sumatraExe) { throw "SumatraPDF.exe not found in zip or local install" }
Copy-Item -Force $sumatraExe.FullName (Join-Path $Dist "SumatraPDF.exe")

$nssmZip = Join-Path $env:TEMP "nssm-2.24.zip"
$nssmUrl = "https://nssm.cc/release/nssm-2.24.zip"
Write-Host "==> Downloading nssm"
try {
  Invoke-WebRequest -Uri $nssmUrl -OutFile $nssmZip -UseBasicParsing
} catch {
  # fallback mirror sometimes used by packagers
  Invoke-WebRequest -Uri "https://github.com/kirillkovalenko/nssm/releases/download/2.24-101-g897c7ad/nssm-2.24-101-g897c7ad.zip" -OutFile $nssmZip -UseBasicParsing
}
$nssmExtract = Join-Path $env:TEMP "nssm-extract"
Remove-Item -Recurse -Force $nssmExtract -ErrorAction SilentlyContinue
Expand-Archive -Path $nssmZip -DestinationPath $nssmExtract -Force
$nssmExe = Get-ChildItem $nssmExtract -Recurse -Filter "nssm.exe" | Where-Object { $_.FullName -match "win64|64" } | Select-Object -First 1
if (-not $nssmExe) {
  $nssmExe = Get-ChildItem $nssmExtract -Recurse -Filter "nssm.exe" | Select-Object -First 1
}
if (-not $nssmExe) { throw "nssm.exe not found" }
Copy-Item -Force $nssmExe.FullName (Join-Path $Dist "nssm.exe")

$iscc = @(
  "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe",
  "$env:ProgramFiles\Inno Setup 6\ISCC.exe",
  "${env:ProgramFiles(x86)}\Inno Setup 7\ISCC.exe",
  "$env:ProgramFiles\Inno Setup 7\ISCC.exe",
  "$env:LOCALAPPDATA\Programs\Inno Setup 6\ISCC.exe",
  "$env:LOCALAPPDATA\Programs\Inno Setup 7\ISCC.exe"
) | Where-Object { Test-Path $_ } | Select-Object -First 1

if (-not $iscc) {
  Write-Host "==> Installing Inno Setup via winget"
  winget install --id JRSoftware.InnoSetup -e --accept-package-agreements --accept-source-agreements
  $iscc = @(
    "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe",
    "$env:ProgramFiles\Inno Setup 6\ISCC.exe",
    "$env:LOCALAPPDATA\Programs\Inno Setup 6\ISCC.exe"
  ) | Where-Object { Test-Path $_ } | Select-Object -First 1
}
if (-not $iscc) { throw "ISCC.exe not found. Install Inno Setup 6+." }

Write-Host "==> Compiling installer with $iscc"
& $iscc (Join-Path $PSScriptRoot "print-agent.iss")
if ($LASTEXITCODE -ne 0) { throw "Inno Setup compile failed" }

$out = Join-Path $Dist "OdooPrintAgentSetup.exe"
if (-not (Test-Path $out)) { throw "Expected output missing: $out" }
Write-Host ""
Write-Host "OK: $out"
Get-Item $out | Format-List FullName, Length, LastWriteTime
