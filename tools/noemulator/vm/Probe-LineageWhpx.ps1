param(
    [ValidateRange(10,300)][int]$BootSeconds = 90,
    [switch]$KeepRunning,
    [string]$WorkDirectory,
    [string]$Lab = (Join-Path $env:LOCALAPPDATA 'NoEmulatorLab')
)

# Boots the source LineageOS image as a disposable overlay, verifies it reaches
# QMP, optionally dumps a boot screenshot and quits. Used to prove the WHPX
# AArch64 guest boots before building the persistent snapchat-test overlay.

$ErrorActionPreference = 'Stop'
$lineage = Join-Path $Lab 'lineage'
$qemu = Join-Path $Lab 'runtime\clangarm64\bin\qemu-system-aarch64.exe'
$imageTool = Join-Path (Split-Path $qemu) 'qemu-img.exe'
$firmware = Join-Path $Lab 'runtime\clangarm64\share\qemu\edk2-aarch64-code.fd'
$source = Join-Path $lineage 'LineageOS_on_arm64.utm\Data'
if (-not $WorkDirectory) { $WorkDirectory = Join-Path $lineage 'work' }
$work = [IO.Path]::GetFullPath($WorkDirectory)
New-Item -ItemType Directory -Force $work | Out-Null
foreach ($name in @('vda.qcow2', 'vdb.qcow2', 'efi_vars.fd')) {
    $overlay = Join-Path $work $name
    if (-not (Test-Path $overlay)) {
        & $imageTool create -f qcow2 -F qcow2 -b (Join-Path $source $name) $overlay
        if ($LASTEXITCODE -ne 0) { throw "Cannot create $name overlay" }
    }
}
$prefix = Join-Path $work ('boot-' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
$listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0)
$listener.Start()
$port = $listener.LocalEndpoint.Port
$process = $null
$client = $null
try {
    $arguments = @(
        '-accel', 'whpx', '-machine', 'virt', '-cpu', 'host', '-smp', '4', '-m', '4096',
        '-drive', "if=pflash,unit=0,format=raw,readonly=on,file=$firmware",
        '-drive', "if=pflash,unit=1,format=qcow2,file=$work\efi_vars.fd",
        '-drive', "file=$work\vda.qcow2,if=none,id=vda,format=qcow2",
        '-device', 'virtio-blk-pci,drive=vda,bootindex=0',
        '-drive', "file=$work\vdb.qcow2,if=none,id=vdb,format=qcow2",
        '-device', 'virtio-blk-pci,drive=vdb,bootindex=1',
        '-netdev', 'user,id=net0,hostfwd=tcp:127.0.0.1:15555-:5555',
        '-device', 'virtio-net-pci,netdev=net0',
        '-device', 'virtio-gpu-pci', '-display', 'sdl,gl=off',
        '-device', 'qemu-xhci,id=xhci', '-device', 'usb-tablet,bus=xhci.0',
        '-device', 'usb-kbd,bus=xhci.0', '-device', 'virtio-rng-pci',
        '-device', 'virtio-serial-pci,id=serial0',
        '-chardev', "file,id=shell,path=$prefix.shell.log",
        '-device', 'virtconsole,chardev=shell',
        '-chardev', "file,id=logcat,path=$prefix.logcat.log",
        '-device', 'virtconsole,chardev=logcat',
        '-serial', "file:$prefix.serial.log", '-monitor', 'none',
        '-qmp', "tcp:127.0.0.1:$port",
        '-qmp', 'tcp:127.0.0.1:15556,server=on,wait=off', '-no-reboot'
    )
    $process = Start-Process $qemu -ArgumentList $arguments -PassThru -RedirectStandardOutput "$prefix.stdout.log" -RedirectStandardError "$prefix.stderr.log"
    $accept = $listener.AcceptTcpClientAsync()
    if (-not $accept.Wait(20000)) { throw "QMP connection timeout; inspect $prefix.stderr.log" }
    $client = $accept.Result
    $stream = $client.GetStream()
    $stream.ReadTimeout = 10000
    $reader = [IO.StreamReader]::new($stream)
    $writer = [IO.StreamWriter]::new($stream)
    $writer.AutoFlush = $true
    $reader.ReadLine() | Out-File "$prefix.qmp.log"
    function Invoke-Qmp([string]$Command, [hashtable]$Arguments = @{}) {
        $writer.WriteLine((@{execute=$Command; arguments=$Arguments} | ConvertTo-Json -Compress -Depth 5))
        do {
            $line = $reader.ReadLine()
            if ($null -eq $line) { throw 'QMP closed' }
            Add-Content "$prefix.qmp.log" $line
            $response = $line | ConvertFrom-Json
            if ($response.PSObject.Properties['error']) { throw $line }
        } until ($response.PSObject.Properties['return'])
        $response.return
    }
    Invoke-Qmp 'qmp_capabilities' | Out-Null
    Invoke-Qmp 'query-status' | Format-List
    if ($KeepRunning) {
        Write-Output "LIVE_PID $($process.Id); LOG_PREFIX $prefix; QMP 127.0.0.1:15556"
        $process.WaitForExit()
        if ($process.ExitCode -ne 0) { throw "QEMU exit code $($process.ExitCode)" }
        return
    }
    if ($process.WaitForExit($BootSeconds * 1000)) { throw "QEMU exited early: $($process.ExitCode); inspect $prefix.stderr.log" }
    Invoke-Qmp 'screendump' @{filename="$prefix.png"; format='png'} | Out-Null
    Invoke-Qmp 'query-status' | Format-List
    Write-Output "CAPTURE $prefix.png"
    Invoke-Qmp 'quit' | Out-Null
    if (-not $process.WaitForExit(15000)) { throw 'QEMU did not exit' }
    if ($process.ExitCode -ne 0) { throw "QEMU exit code $($process.ExitCode)" }
    Write-Output "LOG_PREFIX $prefix"
} finally {
    if ($client) { $client.Dispose() }
    $listener.Stop()
    if ($process -and -not $process.HasExited) { $process.Kill(); $process.WaitForExit() }
}
