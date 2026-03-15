$ErrorActionPreference = "Stop"

Write-Host "=== INSTALLING SIMPLYSIGN DESKTOP ==="

$CertumInstaller = "SimplySignDesktop.msi"

# Find link here: https://support.certum.eu/en/software/procertum-smartsign/
$DownloadUrl = "https://files.certum.eu/software/SimplySignDesktop/Windows/9.4.2.86/SimplySignDesktop-9.4.2.86-64-bit-en.msi"

# (Get-FileHash -Path .\SimplySignDesktop-9.4.2.86-64-bit-en.msi -Algorithm SHA256).Hash.ToLower()
$ExpectedChecksum = "0f8f386484e2c30882dae35961e662fdbfc23e305a5ca3639ca586b68a92bd83"

Write-Host "Downloading SimplySign Desktop MSI..."
try {
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $CertumInstaller -TimeoutSec 600
    $fileSize = (Get-Item $CertumInstaller).Length / 1MB
    Write-Host "Downloaded SimplySign Desktop MSI ($([math]::Round($fileSize, 1)) MB)"
} catch {
    Write-Host "Failed to download SimplySign Desktop: $($_.Exception.Message)"
    exit 1
}

# Verify checksum
$ActualChecksum = (Get-FileHash -Path $CertumInstaller -Algorithm SHA256).Hash.ToLower()
if ($ExpectedChecksum -ne $ActualChecksum) {
    Write-Host "Checksum verification failed"
    Write-Host "Expected: $ExpectedChecksum"
    Write-Host "Actual:   $ActualChecksum"
    exit 1
}
Write-Host "Checksum verified"

# Check for administrative privileges
$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if ($isAdmin) {
    Write-Host "Running with administrative privileges"
} else {
    Write-Host "No explicit administrative privileges detected"
}

# Install SimplySign Desktop
Write-Host "Installing SimplySign Desktop..."
$installArgs = "/i `"$CertumInstaller`" /quiet /norestart /l*v install.log ALLUSERS=1 REBOOT=ReallySuppress"
$process = Start-Process -FilePath "msiexec.exe" -ArgumentList $installArgs -Wait -NoNewWindow -PassThru

if ($process.ExitCode -ne 0) {
    Write-Host "MSI installation returned exit code: $($process.ExitCode)"
}

# Verify installation via log patterns
$installationSuccessful = $false
if (Test-Path "install.log") {
    $logContent = Get-Content "install.log" -Raw -ErrorAction SilentlyContinue
    if ($logContent -match "Installation.*operation.*completed.*successfully|Installation.*success.*or.*error.*status.*0|MainEngineThread.*is.*returning.*0|Windows.*Installer.*installed.*the.*product") {
        Write-Host "Installation successful (confirmed by log patterns)"
        $installationSuccessful = $true
    }
}

# Verify installation directory
$InstallPath = "C:\Program Files\Certum\SimplySign Desktop"
if (Test-Path $InstallPath) {
    Write-Host "SimplySign Desktop installed successfully"
    Write-Host "Virtual card emulation now active for code signing"
    $installationSuccessful = $true

    if ($env:GITHUB_OUTPUT) {
        "SIMPLYSIGN_PATH=$InstallPath" | Out-File -FilePath $env:GITHUB_OUTPUT -Append -Encoding UTF8
    }
}

if (-not $installationSuccessful) {
    Write-Host "Installation verification failed"
    if (Test-Path "install.log") {
        Write-Host "Last 10 lines of install log:"
        Get-Content "install.log" -Tail 10
    } else {
        Write-Host "No install log available"
    }
    exit 1
}

Write-Host "SimplySign Desktop installation completed successfully!"
