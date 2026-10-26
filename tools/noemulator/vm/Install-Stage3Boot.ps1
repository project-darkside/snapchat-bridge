param(
    [string]$Lab = (Join-Path $env:LOCALAPPDATA 'NoEmulatorLab')
)

# One-time offline maintenance: boots an Alpine rescue image and flashes the
# patched boot image into the stage3-test guest's boot partition. Requires the
# lab's images\boot\{vmlinuz-virt,initramfs-virt}, images\alpine-virt-*.iso, and
# a lineage\stage3-test overlay with its boot-input directory.

$ErrorActionPreference = 'Stop'
$lineage = Join-Path $Lab 'lineage'
$work = Join-Path $lineage 'stage3-test'
$qemu = Join-Path $Lab 'runtime\clangarm64\bin\qemu-system-aarch64.exe'
$active = Get-CimInstance Win32_Process -Filter "Name='qemu-system-aarch64.exe'" | Where-Object { $_.CommandLine -like "*$work*" }
if ($active) { throw 'Stop the stage3-test guest before offline maintenance' }
$listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 0)
$listener.Start()
$port = $listener.LocalEndpoint.Port
$process = $null
$client = $null
$stream = $null
$log = [System.IO.StreamWriter]::new((Join-Path $work 'boot-install.serial.log'), $false)
function Read-Until {
    param([string]$Pattern)
    $received = [System.Text.StringBuilder]::new()
    $buffer = [byte[]]::new(4096)
    while ($true) {
        $count = $stream.Read($buffer, 0, $buffer.Length)
        if ($count -eq 0) { throw "Serial closed before $Pattern" }
        $text = [System.Text.Encoding]::ASCII.GetString($buffer, 0, $count)
        $log.Write($text)
        $log.Flush()
        [void]$received.Append($text)
        if ($received.ToString() -match $Pattern) { return $received.ToString() }
    }
}
function Send-Command {
    param([string]$Command)
    $bytes = [System.Text.Encoding]::ASCII.GetBytes("$Command`n")
    $stream.Write($bytes, 0, $bytes.Length)
    $stream.Flush()
}
try {
    $arguments = @(
        '-accel', 'whpx', '-machine', 'virt', '-cpu', 'host', '-smp', '2', '-m', '2048',
        '-kernel', "$Lab\images\boot\vmlinuz-virt", '-initrd', "$Lab\images\boot\initramfs-virt",
        '-append', '"console=ttyAMA0 modules=loop,squashfs,sd-mod,usb-storage,virtio_blk,vfat"',
        '-device', 'virtio-scsi-pci,id=scsi0',
        '-drive', "file=$Lab\images\alpine-virt-3.24.3-aarch64.iso,if=none,id=cd0,format=raw,media=cdrom,readonly=on",
        '-device', 'scsi-cd,drive=cd0',
        '-drive', "file=$work\vda.qcow2,if=virtio,format=qcow2",
        '-drive', "file=fat:ro:$work\boot-input,if=virtio,format=raw,readonly=on",
        '-nic', 'none', '-display', 'none', '-monitor', 'none',
        '-serial', "tcp:127.0.0.1:$port", '-no-reboot'
    )
    $process = Start-Process $qemu -ArgumentList $arguments -PassThru -NoNewWindow `
        -RedirectStandardOutput (Join-Path $work 'boot-install.qemu.stdout.log') `
        -RedirectStandardError (Join-Path $work 'boot-install.qemu.stderr.log')
    $null = $process.Handle
    $connection = $listener.AcceptTcpClientAsync()
    if (-not $connection.Wait(30000)) { throw 'Maintenance serial connection timed out' }
    $client = $connection.Result
    $listener.Stop()
    $stream = $client.GetStream()
    $stream.ReadTimeout = 120000
    [void](Read-Until 'localhost login: ')
    Send-Command 'root'
    [void](Read-Until 'localhost:~# ')
    Send-Command 'stty -echo'
    [void](Read-Until 'localhost:~# ')
    Send-Command '(set -e; mkdir -p /mnt/input; mount -t vfat -o ro /dev/vdb1 /mnt/input; actual=$(head -c 46219264 /dev/vda6 | sha256sum); test "${actual%% *}" = d3efda5df6e2bdd152de6814bffc0b9050925ddd7f3c114cd3dd68c02a758cf5; echo STOCK_BOOT_MATCH; size=$(stat -c%s /mnt/input/patched-boot.img); test "$size" -lt "$(blockdev --getsize64 /dev/vda6)"; dd if=/mnt/input/patched-boot.img of=/dev/vda6 bs=4096 conv=fsync; expected=$(sha256sum /mnt/input/patched-boot.img); actual=$(head -c "$size" /dev/vda6 | sha256sum); test "${actual%% *}" = "${expected%% *}"; echo STAGE3_BOOT_INSTALLED)'
    $result = Read-Until 'localhost:~# '
    Write-Output $result.Trim()
    Send-Command 'sync; poweroff'
    [void](Read-Until 'reboot: Power down')
    if (-not $process.WaitForExit(60000)) { throw 'Maintenance guest did not power off' }
    if ($process.ExitCode -ne 0) { throw "Maintenance QEMU exit $($process.ExitCode)" }
    if ($result -notmatch '(?m)^STAGE3_BOOT_INSTALLED\r?$') { throw 'Boot installation did not pass its checks' }
    Write-Output 'MAINTENANCE_POWEROFF_EXIT_0'
}
finally {
    if ($stream) { $stream.Dispose() }
    if ($client) { $client.Dispose() }
    $listener.Stop()
    $log.Dispose()
    if ($process) {
        if (-not $process.HasExited) { $process.Kill(); $process.WaitForExit() }
        $process.Dispose()
    }
}
