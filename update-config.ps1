Param(
    [string]$ConfigPath = (Join-Path $PSScriptRoot 'config.yaml'),
    [switch]$WhatIf
)

Write-Host "Using config file: $ConfigPath"

if (-not (Test-Path -LiteralPath $ConfigPath)) {
    Write-Error "Config file not found at '$ConfigPath'"
    exit 1
}

if (-not (Get-Command Get-AudioDevice -ErrorAction SilentlyContinue)) {
    Write-Error "The 'Get-AudioDevice' cmdlet is not available. Install/import AudioDeviceCmdlets first."
    exit 1
}

Write-Host "Querying audio devices..."
$devices = Get-AudioDevice -List

if (-not $devices) {
    Write-Error "No audio devices returned by Get-AudioDevice -List."
    exit 1
}

# Read config as individual lines so we don't create extra blank lines.
$lines = Get-Content -LiteralPath $ConfigPath -ErrorAction Stop -Encoding UTF8

# Track current combo number for nicer messages.
$currentCombo = $null

$updatedLines = $lines | ForEach-Object {
    $line = $_

    # Remember current combo number, e.g. "  - combo: 0"
    if ($line -match '^\s*-\s*combo:\s*(\d+)\s*$') {
        $currentCombo = [int]$Matches[1]
        return $line
    }

    # Look for lines like: deviceID: "..." # Comment with friendly name
    if ($line -match '^(?<prefix>\s*deviceID:\s*)"(?<id>[^"]*)"(?<suffix>.*)$') {
        $prefix = $Matches['prefix']
        $existingId = $Matches['id']
        $suffix = $Matches['suffix']

        # Try to extract a friendly name from an inline comment, if present.
        $friendlyName = $null
        if ($suffix -match '#\s*(?<name>.+)$') {
            $friendlyName = ($Matches['name']).Trim()
        }

        if (-not $friendlyName) {
            # No comment / friendly name to match on – leave line unchanged.
            return $line
        }

        # Find the device(s) whose Name contains the friendly name.
        $matches = $devices | Where-Object { $_.Name -like "*$friendlyName*" }

        if ($matches.Count -eq 1) {
            $device = $matches[0]
            $newId = $device.Id

            if ($newId -and $newId -ne $existingId) {
                $comboText = if ($null -ne $currentCombo) { "combo $currentCombo" } else { "unknown combo" }
                Write-Host "Updating $comboText for '$friendlyName'" -ForegroundColor Cyan
                Write-Host "  $existingId" -ForegroundColor DarkGray
                Write-Host "  -> $newId" -ForegroundColor Green

                if ($WhatIf) {
                    # Simulate change only.
                    return $line
                }

                # Rebuild the line with the new ID, preserving comments and spacing.
                return "$prefix`"$newId`"$suffix"
            }
            else {
                # Either same ID or no new ID – leave as is.
                return $line
            }
        }
        elseif ($matches.Count -gt 1) {
            $comboText = if ($null -ne $currentCombo) { "combo $currentCombo" } else { "unknown combo" }
            Write-Warning "Multiple device matches for '$friendlyName' ($comboText). Leaving line unchanged."
            return $line
        }
        else {
            $comboText = if ($null -ne $currentCombo) { "combo $currentCombo" } else { "unknown combo" }
            Write-Warning "No device matches for '$friendlyName' ($comboText). Leaving line unchanged."
            return $line
        }
    }

    return $line
}

if ($WhatIf) {
    Write-Host ""
    Write-Host "WhatIf: No changes were written to '$ConfigPath'." -ForegroundColor Yellow
    exit 0
}

# Create a simple backup before overwriting.
$backupPath = "$ConfigPath.bak"
if (-not (Test-Path -LiteralPath $backupPath)) {
    Copy-Item -LiteralPath $ConfigPath -Destination $backupPath -ErrorAction SilentlyContinue
    Write-Host "Backup created at '$backupPath'."
}

# Write updated content back with standard Windows newlines.
($updatedLines -join "`r`n") | Set-Content -LiteralPath $ConfigPath -Encoding UTF8

Write-Host "Config file updated successfully."


