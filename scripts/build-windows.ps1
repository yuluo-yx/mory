$ErrorActionPreference = "Stop"

Write-Host "[Mory] Checking the Node.js environment"
node --version
npm --version

if (-not (Test-Path "node_modules")) {
    Write-Host "[Mory] Installing locked dependencies"
    npm ci
}

Write-Host "[Mory] Running checks and tests"
npm run check
npm test

Write-Host "[Mory] Building Windows installer and portable packages"
npm run pack:win

Write-Host "[Mory] Artifacts: dist/windows"
