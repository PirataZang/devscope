$repo = "PirataZang/devscope"
$version = $env:DEVSCOPE_VERSION
if (-not $version) {
    # Get latest version
    $response = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest"
    $version = $response.tag_name
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
    Write-Error "Arquitetura não suportada: $arch"
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

Write-Host "Baixando $url..."
Invoke-WebRequest -Uri $url -OutFile $zipFile

Write-Host "Extraindo arquivos..."
Expand-Archive -Path $zipFile -DestinationPath $tempDir -Force

Copy-Item -Path (Join-Path $tempDir "devscope.exe") -Destination (Join-Path "$installDir\bin" "devscope.exe") -Force

# Clean up
Remove-Item -Path $zipFile -Force
Remove-Item -Path (Join-Path $tempDir "devscope.exe") -Force
Remove-Item -Path (Join-Path $tempDir "README.md") -Force -ErrorAction SilentlyContinue
Remove-Item -Path (Join-Path $tempDir "configs") -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item -Path (Join-Path $tempDir "docs") -Recurse -Force -ErrorAction SilentlyContinue

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
