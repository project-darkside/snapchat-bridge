param(
    [string]$Name = 'snapchat-test',
    [string]$Base = 'port-test',
    # Root of the NoEmulator lab (contains runtime\, lineage\). Override if
    # the lab lives somewhere other than %LOCALAPPDATA%\NoEmulatorLab.
    [string]$Lab = (Join-Path $env:LOCALAPPDATA 'NoEmulatorLab')
)

# Boots the rooted NoEmulator LineageOS VM (Android 16, KernelSU root) as a
# child overlay of an already-installed base. Exposes:
#   adb: 127.0.0.1:15555 (guest 5555)
#   QMP: 127.0.0.1:15556 (control via Invoke-LineageMonitor.ps1)
#
# Prerequisites: a working lab tree (runtime\clangarm64\bin\qemu-system-aarch64.exe
# and lineage\LineageOS_on_arm64.utm\Data\{vda,vdb,efi_vars}.qcow2) plus a base
# overlay named $Base that already has Snapchat installed and rooted.

$ErrorActionPreference = 'Stop'
$lineage = Join-Path $Lab 'lineage'
$qemu = Join-Path $Lab 'runtime\clangarm64\bin\qemu-system-aarch64.exe'
$imageTool = Join-Path (Split-Path $qemu) 'qemu-img.exe'
$firmware = Join-Path $Lab 'runtime\clangarm64\share\qemu\edk2-aarch64-code.fd'

$work = Join-Path $lineage $Name
$parent = Join-Path $lineage $Base
New-Item -ItemType Directory -Force $work | Out-Null

foreach ($n in @('vda.qcow2', 'vdb.qcow2', 'efi_vars.fd')) {
    $overlay = Join-Path $work $n
    if (-not (Test-Path $overlay)) {
        & $imageTool create -f qcow2 -F qcow2 -b (Join-Path $parent $n) $overlay
        if ($LASTEXITCODE -ne 0) { throw "Cannot create $n overlay" }
    }
}

$prefix = Join-Path $work ('boot-' + (Get-Date -Format 'yyyyMMdd-HHmmss'))

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
    '-qmp', 'tcp:127.0.0.1:15556,server=on,wait=off', '-no-reboot'
)

$process = Start-Process $qemu -ArgumentList $arguments -PassThru `
    -RedirectStandardOutput "$prefix.stdout.log" -RedirectStandardError "$prefix.stderr.log"

Set-Content (Join-Path $work 'qemu.pid') $process.Id
Set-Content (Join-Path $work 'prefix.txt') $prefix
Write-Output "QEMU_PID $($process.Id)"
Write-Output "PREFIX $prefix"
Write-Output "WORK $work"
