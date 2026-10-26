param(
    [switch]$PowerOff,
    [int]$QemuProcessId,
    [string]$Adb = (Join-Path $env:LOCALAPPDATA 'Android\Sdk\platform-tools\adb.exe'),
    [string]$Lab = (Join-Path $env:LOCALAPPDATA 'NoEmulatorLab')
)

# Runs an end-to-end guest verification probe (arch, root uid, boot completed,
# network + DNS + verified HTTPS) against the VM booted by Start-SnapchatGuest.
# With -PowerOff, requests Android poweroff and asserts QEMU exits 0.

$ErrorActionPreference = 'Stop'
$target = '127.0.0.1:15555'
$lineage = Join-Path $Lab 'lineage'
$logDir = Join-Path $lineage 'fresh-test'
New-Item -ItemType Directory -Force $logDir | Out-Null
$log = Join-Path $logDir ('verification-' + (Get-Date -Format 'yyyyMMdd-HHmmss') + '.log')
$process = $null
if ($PowerOff) {
    if ($QemuProcessId -le 0) { throw 'PowerOff requires the owned QEMU process ID' }
    $process = Get-Process -Id $QemuProcessId
    $null = $process.Handle
    $expected = Join-Path $Lab 'runtime\clangarm64\bin\qemu-system-aarch64.exe'
    $details = Get-CimInstance Win32_Process -Filter "ProcessId = $QemuProcessId"
    if ($process.Path -ne $expected -or $details.CommandLine -notmatch 'fresh-test' -or $details.CommandLine -notmatch '127\.0\.0\.1:15555') {
        throw 'Process does not match the isolated fresh-test guest'
    }
}

$probe = @'
set -e
echo AUTHENTICATED_ARM64_ANDROID
id
test "$(id -u)" = 2000
test "$(uname -m)" = aarch64
test "$(getprop sys.boot_completed)" = 1
uname -r
getprop ro.build.fingerprint
echo IP_CONNECTIVITY
ping -c 1 -W 5 1.1.1.1
echo DNS_CONNECTIVITY
ping -c 1 -W 5 example.com
echo CERTIFICATE_VERIFIED_HTTPS
result=$(curl --fail --show-error --silent --max-time 30 --output /dev/null --write-out "%{http_code} %{ssl_verify_result}" https://example.com/)
echo "$result"
test "$result" = "200 0"
echo LIVE_GUEST_CHECKS_PASSED
'@
$probe = $probe.Replace("`r`n", "`n")
$output = & $Adb -s $target shell $probe 2>&1
$probeExit = $LASTEXITCODE
$output | Tee-Object -FilePath $log
if ($probeExit -ne 0) { throw "Guest verification failed; inspect $log" }
if ($PowerOff) {
    & $Adb -s $target shell reboot -p
    if ($LASTEXITCODE -ne 0) { throw 'Android poweroff request failed' }
    if (-not $process.WaitForExit(90000)) { throw 'Android poweroff did not stop QEMU within 90 seconds' }
    if ($process.ExitCode -ne 0) { throw "QEMU poweroff exit code $($process.ExitCode)" }
    'ANDROID_POWEROFF_QEMU_EXIT_0' | Tee-Object -FilePath $log -Append
}
Write-Output "VERIFICATION_LOG $log"
