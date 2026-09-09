$ErrorActionPreference = 'Stop'
python scripts/reuse_native_runtime.py
if ($LASTEXITCODE -ne 0) {
    ./scripts/build_native_windows.ps1
    if ($LASTEXITCODE -ne 0) { throw 'Native runtime build failed' }
    exit 0
}
# Repeat native execution and dependency checks on the recovered bytes.
& native-package/sd-cli.exe --list-devices
if ($LASTEXITCODE -ne 0) { throw 'Recovered native runtime did not start' }
$dumpbin = Get-ChildItem 'C:/Program Files/Microsoft Visual Studio/2022/Enterprise/VC/Tools/MSVC/*/bin/Hostx64/x64/dumpbin.exe' | Select-Object -Last 1
if (-not $dumpbin) { throw 'dumpbin verification unavailable' }
$imports = & $dumpbin.FullName /dependents native-package/sd-cli.exe | Out-String
$imports | Set-Content native-package/imports.txt
if ($imports -match '(?i)MSVCP\d+|VCRUNTIME\d+|vcomp\d+|stable-diffusion\.dll|ggml.*\.dll') { throw 'Unbundled runtime dependency found' }
python scripts/register_bundled_runtime.py
if ($LASTEXITCODE -ne 0) { throw 'Runtime manifest registration failed' }
