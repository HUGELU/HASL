$ErrorActionPreference = 'Stop'
$pin = 'd04e8950c1ec8d30248cbe996682b3182fb1adf6'
git clone --filter=blob:none --no-checkout https://github.com/leejet/stable-diffusion.cpp.git _native-src
if ($LASTEXITCODE -ne 0) { throw 'Backend clone failed' }
git -C _native-src checkout $pin
if ($LASTEXITCODE -ne 0) { throw 'Pinned backend checkout failed' }
git -C _native-src submodule update --init --recursive --depth 1
if ($LASTEXITCODE -ne 0) { throw 'Pinned submodule checkout failed' }
cmake -S _native-src -B native-build -A x64 -DCMAKE_POLICY_DEFAULT_CMP0091=NEW -DCMAKE_MSVC_RUNTIME_LIBRARY=MultiThreaded -DSD_WEBP=OFF -DSD_WEBM=OFF -DSD_BUILD_SHARED_LIBS=OFF -DSD_BUILD_SHARED_GGML_LIB=OFF -DGGML_BACKEND_DL=OFF -DGGML_OPENMP=OFF -DGGML_NATIVE=OFF -DGGML_AVX2=ON
if ($LASTEXITCODE -ne 0) { throw 'Native configure failed' }
cmake --build native-build --config Release --target sd-cli --parallel 4
if ($LASTEXITCODE -ne 0) { throw 'Native compile failed' }
New-Item -ItemType Directory -Force native-package | Out-Null
Copy-Item native-build/bin/Release/sd-cli.exe native-package/sd-cli.exe
Copy-Item _native-src/LICENSE native-package/stable-diffusion.cpp-LICENSE.txt
Copy-Item _native-src/ggml/LICENSE native-package/ggml-LICENSE.txt
Get-ChildItem _native-src/thirdparty -Filter '*license*' -Recurse | ForEach-Object { Copy-Item $_.FullName ('native-package/' + $_.Directory.Name + '-' + $_.Name) }
& native-package/sd-cli.exe --list-devices
if ($LASTEXITCODE -ne 0) { throw 'Compiled CPU runtime failed its startup probe' }
# Inspect actual PE imports; a /MT intention alone is not proof of portability.
$dumpbin = Get-ChildItem 'C:/Program Files/Microsoft Visual Studio/2022/Enterprise/VC/Tools/MSVC/*/bin/Hostx64/x64/dumpbin.exe' | Select-Object -Last 1
if (-not $dumpbin) { throw 'dumpbin import verification unavailable' }
$imports = & $dumpbin.FullName /dependents native-package/sd-cli.exe | Out-String
$imports | Set-Content native-package/imports.txt
if ($imports -match '(?i)MSVCP\d+|VCRUNTIME\d+|vcomp\d+|stable-diffusion\.dll|ggml.*\.dll') { throw 'Portable runtime still imports unbundled runtime DLLs' }
New-Item -ItemType Directory -Force bundled | Out-Null
Compress-Archive -Path native-package/* -DestinationPath bundled/origin0-sd-d04e895-windows-amd64-cpu.zip
python scripts/register_bundled_runtime.py
if ($LASTEXITCODE -ne 0) { throw 'Runtime manifest registration failed' }
