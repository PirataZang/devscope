$ErrorActionPreference = "Stop"

$repo = "PirataZang/devscope"
$version = $env:DEVSCOPE_VERSION

if (-not $version) {
    try {
        $response = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest"
        $version = $response.tag_name
    } catch {
        Write-Host ""
        Write-Host "Erro ao obter a versão mais recente do DevScope: $_" -ForegroundColor Red
        Write-Host ""
        Write-Host "Dica: Isso geralmente acontece porque não há nenhuma release pública criada no repositório ainda." -ForegroundColor Yellow
        Write-Host "Para instalar uma versão específica, defina a variável de ambiente e tente novamente:" -ForegroundColor Yellow
        Write-Host '  $env:DEVSCOPE_VERSION="0.1.0"' -ForegroundColor Green
        Write-Host '  irm https://raw.githubusercontent.com/PirataZang/devscope/main/scripts/install.ps1 | iex' -ForegroundColor Green
        exit 1
    }
}

# Garante que a tag começa com v
if ($version -notlike "v*") {
    $version = "v$version"
}
$ver = $version.Substring(1)

# Detecta arquitetura
$arch = $env:PROCESSOR_ARCHITECTURE
if ($arch -eq "AMD64") {
    $arch = "amd64"
} elseif ($arch -eq "ARM64") {
    $arch = "arm64"
} else {
    Write-Host "Erro: Arquitetura não suportada: $arch" -ForegroundColor Red
    exit 1
}

$asset     = "devscope_${ver}_windows_${arch}.zip"
$url       = "https://github.com/$repo/releases/download/$version/$asset"
$checkUrl  = "https://github.com/$repo/releases/download/$version/checksums.txt"
$installDir = if ($env:DEVSCOPE_INSTALL_DIR) { $env:DEVSCOPE_INSTALL_DIR } else { "$env:USERPROFILE\.devscope" }
$binDir    = "$installDir\bin"

Write-Host ""
Write-Host "==> Instalando DevScope $version ($arch) em $binDir..." -ForegroundColor Cyan

New-Item -ItemType Directory -Force -Path $installDir | Out-Null
New-Item -ItemType Directory -Force -Path $binDir | Out-Null

$tempDir = [System.IO.Path]::GetTempPath()
$zipFile = Join-Path $tempDir $asset

# Download do binário
try {
    Write-Host "==> Baixando $asset..."
    Invoke-WebRequest -Uri $url -OutFile $zipFile -UseBasicParsing
} catch {
    Write-Host ""
    Write-Host "Erro ao baixar o arquivo da release: $_" -ForegroundColor Red
    Write-Host "Verifique se a release '$version' e o arquivo '$asset' existem em:" -ForegroundColor Yellow
    Write-Host "  https://github.com/$repo/releases" -ForegroundColor Yellow
    exit 1
}

# Verificação de checksum (opcional — continua se checksums.txt não existir)
try {
    $checksumFile = Join-Path $tempDir "devscope_checksums.txt"
    Invoke-WebRequest -Uri $checkUrl -OutFile $checksumFile -UseBasicParsing -ErrorAction Stop

    $expected = (Get-Content $checksumFile | Where-Object { $_ -match " $asset$" }) -replace "^(\S+)\s.*$", '$1'
    if ($expected) {
        $actual = (Get-FileHash -Path $zipFile -Algorithm SHA256).Hash.ToLower()
        if ($actual -ne $expected.ToLower()) {
            Write-Host "Erro: checksum inválido — o download pode estar corrompido." -ForegroundColor Red
            Write-Host "  Esperado : $expected" -ForegroundColor Red
            Write-Host "  Obtido   : $actual" -ForegroundColor Red
            exit 1
        }
        Write-Host "==> Checksum verificado ✓" -ForegroundColor Green
    }
    if (Test-Path $checksumFile) { Remove-Item $checksumFile -Force }
} catch {
    # checksums.txt pode não existir (releases antigas ou sem checksum) — ignora silenciosamente
}

# Extração e cópia do binário
try {
    Write-Host "==> Extraindo arquivos..."
    $extractDir = Join-Path $tempDir "devscope_extract_$([System.IO.Path]::GetRandomFileName())"
    New-Item -ItemType Directory -Force -Path $extractDir | Out-Null
    Expand-Archive -Path $zipFile -DestinationPath $extractDir -Force

    # Busca devscope.exe em qualquer subdiretório extraído
    $binarySource = Get-ChildItem -Path $extractDir -Recurse -Filter "devscope.exe" | Select-Object -First 1
    if (-not $binarySource) {
        throw "devscope.exe não encontrado no arquivo extraído."
    }

    Copy-Item -Path $binarySource.FullName -Destination (Join-Path $binDir "devscope.exe") -Force
} catch {
    Write-Host "Erro ao extrair ou instalar o binário: $_" -ForegroundColor Red
    exit 1
} finally {
    if (Test-Path $zipFile)    { Remove-Item $zipFile -Force -ErrorAction SilentlyContinue }
    if (Test-Path $extractDir) { Remove-Item $extractDir -Recurse -Force -ErrorAction SilentlyContinue }
}

Write-Host "==> DevScope instalado com sucesso em $binDir\devscope.exe" -ForegroundColor Green

# Adiciona ao PATH do usuário se ainda não estiver
$currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
$pathParts   = $currentPath -split ";" | Where-Object { $_ -ne "" }
if ($pathParts -notcontains $binDir) {
    [Environment]::SetEnvironmentVariable("Path", ($currentPath.TrimEnd(";") + ";" + $binDir), "User")
    Write-Host ""
    Write-Host "==> $binDir adicionado ao seu PATH de usuário." -ForegroundColor Yellow
    Write-Host "    Reinicie o terminal (ou abra um novo) para usar o comando 'devscope'." -ForegroundColor Yellow
    Write-Host ""
    Write-Host "    Para usar imediatamente nesta sessão:" -ForegroundColor Cyan
    Write-Host "      `$env:PATH += `";$binDir`"" -ForegroundColor Cyan
    Write-Host "      devscope" -ForegroundColor Cyan
} else {
    Write-Host ""
    Write-Host "==> $binDir já está no seu PATH." -ForegroundColor Green
    Write-Host "    Execute: devscope" -ForegroundColor Cyan
}

Write-Host ""
