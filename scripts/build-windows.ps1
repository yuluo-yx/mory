$ErrorActionPreference = "Stop"

Write-Host "[Mory] 检查 Node.js 环境"
node --version
npm --version

if (-not (Test-Path "node_modules")) {
    Write-Host "[Mory] 安装锁定依赖"
    npm ci
}

Write-Host "[Mory] 执行语法检查与测试"
npm run check
npm test

Write-Host "[Mory] 构建 Windows 安装版和便携版"
npm run pack:win

Write-Host "[Mory] 制品目录：dist/windows"
