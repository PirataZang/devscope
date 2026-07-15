$ErrorActionPreference = "Stop"

$repo = "PirataZang/devscope"
$version = $env:DEVSCOPE_VERSION

if (-not $version) {
    try {
        $response = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest"
        $version = $response.tag_name
    } catch {
        Write-Host "Erro ao obter a versão mais recente do DevScope: $_" -ForegroundColor Red
        Write-Host "Dica: Isso geralmente acontece porque não há nenhuma release pública criada no repositório ainda." -ForegroundColor Yellow
        Write-Host "Para corrigir, crie uma release em https://github.com/$repo/releases ou defina a versão manualmente executando:" -ForegroundColor Yellow
        Write-Host "  `$env:DEVSCOPE_VERSION='0.1.0' ; irm https://raw.githubusercontent.com/$repo/main/scripts/install.ps1 | iex" -ForegroundColor Green
        exit 1
    }
}

# Ensure tag format starts with v
if ($version -notlike "v*") {
    $version = "v$version"
}
$ver = $version.Substring(1)

# Check architecture
$arch = $env:PROCESSOR_ARCHITECTURE
if ($arch -eq "AMD64") {
    $arch = "amd64"
} elseif ($arch -eq "ARM64") {
    $arch = "arm64"
} else {
    Write-Host "Erro: Arquitetura não suportada: $arch" -ForegroundColor Red
    exit 1
}

$asset = "devscope_${ver}_windows_${arch}.zip"
$url = "https://github.com/$repo/releases/download/$version/$asset"
$installDir = if ($env:DEVSCOPE_INSTALL_DIR) { $env:DEVSCOPE_INSTALL_DIR } else { "$env:USERPROFILE\.devscope" }

Write-Host "Instalando DevScope $version em $installDir..."
New-Item -ItemType Directory -Force -Path $installDir | Out-Null
New-Item -ItemType Directory -Force -Path "$installDir\bin" | Out-Null

$tempDir = [System.IO.Path]::GetTempPath()
$zipFile = Join-Path $tempDir $asset

try {
    Write-Host "Baixando $url..."
    Invoke-WebRequest -Uri $url -OutFile $zipFile
} catch {
    Write-Host "Erro ao baixar o arquivo da release: $_" -ForegroundColor Red
    Write-Host "Verifique se a release '$version' e o arquivo '$asset' existem em https://github.com/$repo/releases" -ForegroundColor Yellow
    exit 1
}

try {
    Write-Host "Extraindo arquivos..."
    Expand-Archive -Path $zipFile -DestinationPath $tempDir -Force
    Copy-Item -Path (Join-Path $tempDir "devscope.exe") -Destination (Join-Path "$installDir\bin" "devscope.exe") -Force
} catch {
    Write-Host "Erro ao extrair ou instalar o binário: $_" -ForegroundColor Red
    exit 1
} finally {
    # Clean up safely
    if (Test-Path $zipFile) { Remove-Item -Path $zipFile -Force }
    if (Test-Path (Join-Path $tempDir "devscope.exe")) { Remove-Item -Path (Join-Path $tempDir "devscope.exe") -Force }
    if (Test-Path (Join-Path $tempDir "README.md")) { Remove-Item -Path (Join-Path $tempDir "README.md") -Force -ErrorAction SilentlyContinue }
    if (Test-Path (Join-Path $tempDir "configs")) { Remove-Item -Path (Join-Path $tempDir "configs") -Recurse -Force -ErrorAction SilentlyContinue }
    if (Test-Path (Join-Path $tempDir "docs")) { Remove-Item -Path (Join-Path $tempDir "docs") -Recurse -Force -ErrorAction SilentlyContinue }
}

Write-Host "DevScope instalado com sucesso!" -ForegroundColor Green

# Add to PATH if not already there
$binPath = "$installDir\bin"
$path = [Environment]::GetEnvironmentVariable("Path", "User")
if ($path -split ";" -notcontains $binPath) {
    [Environment]::SetEnvironmentVariable("Path", "$path;$binPath", "User")
    Write-Host "Adicionado $binPath ao seu PATH de usuário." -ForegroundColor Yellow
    Write-Host "Por favor, reinicie seu terminal para aplicar as mudanças." -ForegroundColor Yellow
} else {
    Write-Host "$binPath já está no seu PATH."
}
