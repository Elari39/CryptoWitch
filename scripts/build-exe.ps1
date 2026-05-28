[CmdletBinding()]
param(
  [ValidateSet("amd64", "arm64")]
  [string]$Arch = "amd64",

  [ValidateSet("pnpm", "npm", "yarn", "bun")]
  [string]$PackageManager = $(if ($env:PACKAGE_MANAGER) { $env:PACKAGE_MANAGER } else { "pnpm" }),

  [switch]$SkipPackDocs,
  [switch]$SkipTests,
  [switch]$Dev
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Get-RepoRoot {
  $scriptDir = Split-Path -Parent $PSCommandPath
  return (Resolve-Path (Join-Path $scriptDir "..")).Path
}

function Require-Command {
  param([string]$Name)

  if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
    throw "Required command not found: $Name"
  }
}

function Invoke-Step {
  param(
    [string]$Name,
    [string]$FilePath,
    [string[]]$Arguments
  )

  Write-Host ""
  Write-Host "==> $Name"
  & $FilePath @Arguments
  if ($LASTEXITCODE -ne 0) {
    throw "$Name failed with exit code $LASTEXITCODE"
  }
}

function Set-VaultPasswordForBuild {
  if ($SkipPackDocs -or -not [string]::IsNullOrWhiteSpace($env:CRYPTOWITCH_VAULT_PASSWORD)) {
    return $false
  }

  $securePassword = Read-Host "Enter CRYPTOWITCH_VAULT_PASSWORD" -AsSecureString
  if ($securePassword.Length -eq 0) {
    throw "CRYPTOWITCH_VAULT_PASSWORD is required unless -SkipPackDocs is used."
  }

  $passwordPointer = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($securePassword)
  try {
    $env:CRYPTOWITCH_VAULT_PASSWORD = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($passwordPointer)
  } finally {
    [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($passwordPointer)
  }
  return $true
}

$repoRoot = Get-RepoRoot
$artifactPath = Join-Path $repoRoot "bin\CryptoWitch.exe"
$originalVaultPassword = $env:CRYPTOWITCH_VAULT_PASSWORD
$scriptSetVaultPassword = $false

try {
  Set-Location $repoRoot

  Require-Command "go"
  Require-Command $PackageManager

  $scriptSetVaultPassword = Set-VaultPasswordForBuild

  if (-not $SkipPackDocs) {
    Invoke-Step "Generate encrypted vault" "go" @("run", "./cmd/packdocs")
  } else {
    Write-Host ""
    Write-Host "==> Skip encrypted vault generation"
  }

  if (-not $SkipTests) {
    Invoke-Step "Run Go tests" "go" @("test", "./...")
  } else {
    Write-Host ""
    Write-Host "==> Skip Go tests"
  }

  $wailsArgs = @(
    "run",
    "github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha.96",
    "task",
    "windows:build",
    "ARCH=$Arch",
    "PACKAGE_MANAGER=$PackageManager"
  )
  if ($Dev) {
    $wailsArgs += "DEV=true"
  }
  Invoke-Step "Build Windows exe" "go" $wailsArgs

  if (-not (Test-Path -LiteralPath $artifactPath)) {
    throw "Build finished but artifact was not found: $artifactPath"
  }

  $artifact = Get-Item -LiteralPath $artifactPath
  $sizeMb = [Math]::Round($artifact.Length / 1MB, 2)

  Write-Host ""
  Write-Host "Build completed."
  Write-Host "Artifact: $($artifact.FullName)"
  Write-Host "Size: $sizeMb MB"
  Write-Host "Updated: $($artifact.LastWriteTime)"
} finally {
  if ($scriptSetVaultPassword) {
    if ([string]::IsNullOrEmpty($originalVaultPassword)) {
      Remove-Item Env:\CRYPTOWITCH_VAULT_PASSWORD -ErrorAction SilentlyContinue
    } else {
      $env:CRYPTOWITCH_VAULT_PASSWORD = $originalVaultPassword
    }
  }
}
