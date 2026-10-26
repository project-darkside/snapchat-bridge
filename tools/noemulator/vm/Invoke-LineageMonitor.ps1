param(
    [string]$Command = 'query-status',
    [hashtable]$Arguments = @{},
    [int[]]$Tap,
    [int]$QmpPort = 15556
)

# Sends a QMP command to the VM launched by Start-SnapchatGuest.ps1 and prints
# the JSON `return` value. Useful commands:
#   -Command 'human-monitor-command' -Arguments @{'command-line'='...'}  (HMP, e.g. hostfwd)
#   -Command 'screendump' -Arguments @{filename='C:\path\screen.png'; format='png'}
#   -Tap 640,360  (synthesize a tap at x,y in the 1280x800 guest)
#
# Example: Invoke-LineageMonitor.ps1 -Command screendump -Arguments @{filename='C:\tmp\s.png';format='png'}

$ErrorActionPreference = 'Stop'
$client = [Net.Sockets.TcpClient]::new('127.0.0.1', $QmpPort)
try {
    $stream = $client.GetStream()
    $stream.ReadTimeout = 15000
    $reader = [IO.StreamReader]::new($stream)
    $writer = [IO.StreamWriter]::new($stream)
    $writer.AutoFlush = $true
    $null = $reader.ReadLine()
    $requests = @(@{execute='qmp_capabilities'}, @{execute=$Command; arguments=$Arguments})
    if ($Tap) {
        if ($Tap.Count -ne 2 -or $Tap[0] -lt 0 -or $Tap[0] -ge 1280 -or $Tap[1] -lt 0 -or $Tap[1] -ge 800) { throw 'Tap requires x,y within 1280x800' }
        $requests = @(
            @{execute='qmp_capabilities'},
            @{execute='input-send-event'; arguments=@{events=@(
                @{type='abs'; data=@{axis='x'; value=[int]($Tap[0] * 32767 / 1279)}},
                @{type='abs'; data=@{axis='y'; value=[int]($Tap[1] * 32767 / 799)}},
                @{type='btn'; data=@{button='left'; down=$true}}
            )}},
            @{execute='input-send-event'; arguments=@{events=@(@{type='btn'; data=@{button='left'; down=$false}})}})
    }
    foreach ($request in $requests) {
        $writer.WriteLine(($request | ConvertTo-Json -Compress -Depth 8))
        do {
            $line = $reader.ReadLine()
            if ($null -eq $line) { throw 'QMP closed' }
            $response = $line | ConvertFrom-Json
            if ($response.PSObject.Properties['error']) { throw $line }
        } until ($response.PSObject.Properties['return'])
    }
    $response.return
} finally {
    $client.Dispose()
}
