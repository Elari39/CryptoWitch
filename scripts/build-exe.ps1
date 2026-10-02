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

function Assert-AccessConfig {
  param([string]$Path)

  if (Test-Path -LiteralPath $Path) {
    return
  }
  throw "Access config not found: $Path. Copy access.example.yaml to access.yaml and fill in the password and MAC whitelist."
}

$repoRoot = Get-RepoRoot
$artifactPath = Join-Path $repoRoot "bin\CryptoWitch.exe"
$accessPath = Join-Path $repoRoot "access.yaml"
$initialLocation = Get-Location

try {
  Set-Location $repoRoot

  Require-Command "go"
  Require-Command $PackageManager

  if (-not $SkipPackDocs) {
    Assert-AccessConfig $accessPath
    Invoke-Step "Generate encrypted vault" "go" @("run", "./cmd/packdocs")
  } else {
    Write-Host ""
    Write-Host "==> Skip encrypted vault generation"
  }

  # 首次构建时 dist 尚不存在，根 Go 包无法编译；先使用仓库内已提交的
  # bindings 构建前端，再执行 Go 测试和 Wails bindings 生成。
  Push-Location (Join-Path $repoRoot "frontend")
  try {
    Invoke-Step "Install frontend dependencies" $PackageManager @("install")
    $frontendBuild = if ($Dev) { "build:dev" } else { "build" }
    Invoke-Step "Build frontend assets" $PackageManager @("run", $frontendBuild)
    if (-not $SkipTests) {
      Invoke-Step "Run frontend tests" $PackageManager @("run", "test")
    }
  } finally {
    Pop-Location
  }

  if (-not $SkipTests) {
    Invoke-Step "Run Go tests" "go" @("test", "./...")
  } else {
    Write-Host ""
    Write-Host "==> Skip Go and frontend tests (type checking remains part of the frontend build)"
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
  Set-Location $initialLocation
}
