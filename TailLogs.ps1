# --- FIX 3: Relaunch in a persistent PowerShell console when double-clicked ---
if ($Host.Name -ne 'ConsoleHost') {
    Start-Process powershell -ArgumentList "-NoExit", "-File `"$PSCommandPath`""
    exit
}

# --- ENSURE ERRORS SHOW ---
$ErrorActionPreference = "Continue"

# --- SET TERMINAL COLORS ---
$Host.UI.RawUI.BackgroundColor = "Black"
$Host.UI.RawUI.ForegroundColor = "Green"
Clear-Host

# --- DEFAULT LOG DIRECTORY ---
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$tailDir = Join-Path $scriptDir "build\logs"


Write-Host "Koolo Tail Script Loaded"
Write-Host "-----------------------------------------`n"

# --- STARTUP INPUT LOOP ---
while ($true) {
    Write-Host "Current log directory:"
    Write-Host "  $tailDir"
    Write-Host "-----------------------------------------"
    Write-Host "Press ENTER to continue"
    Write-Host "Type: cd <path>   to change directory"
    Write-Host "Type: exit        to quit"

    $input = Read-Host ">"

    # exit
    if ($input -ieq "exit") { exit }

    # cd <path>
    if ($input.StartsWith("cd ", [StringComparison]::InvariantCultureIgnoreCase)) {
        $new = $input.Substring(3).Trim()

        if (Test-Path $new -PathType Container) {
            $tailDir = $new
            Write-Host "`nDirectory changed to: $tailDir`n"
        } else {
            Write-Host "`nDirectory does not exist: $new`n" -ForegroundColor Red
        }
        continue
    }

    # Enter -> continue
    if ([string]::IsNullOrWhiteSpace($input)) { break }

    Write-Host "`nInvalid input.`n"
}

# --- FIND AUTOMATIC FILE ---
Write-Host "`nSearching for newest file containing: supervisor-log-"

$autoFile = Get-ChildItem -Path $tailDir -File |
    Where-Object { $_.Name -like "*supervisor-log-*" } |
    Sort-Object LastWriteTime -Descending |
    Select-Object -First 1
	
if ($autoFile) {
    Write-Host "Auto-selected file:"
    Write-Host "  $($autoFile.Name)"
} else {
    Write-Host "No matching log file found!" -ForegroundColor Red
}

# --- ALLOW MANUAL FILE OVERRIDE ---
Write-Host "`nPress ENTER to use this file"
Write-Host "Or type a file name to override:"
$manual = Read-Host "File"

if (-not [string]::IsNullOrWhiteSpace($manual)) {
    $manualPath = Join-Path $tailDir $manual
    if (-not (Test-Path $manualPath -PathType Leaf)) {
        Write-Host "`nERROR: File not found:"
        Write-Host "  $manualPath" -ForegroundColor Red
        Read-Host "`nPress ENTER to close"
        exit
    }
    $fileToTail = $manualPath
} else {
    if (-not $autoFile) {
        Write-Host "`nERROR: No auto-selected file and no manual file provided." -ForegroundColor Red
        Read-Host "`nPress ENTER to close"
        exit
    }
    $fileToTail = $autoFile.FullName
}

Write-Host "`nTailing file:"
Write-Host "  $fileToTail"
Write-Host "-----------------------------------------`n"

# --- EXECUTE TAIL ---
Get-Content -Path $fileToTail -Tail 50 -Wait

Write-Host "`nScript finished. Press ENTER to close."
Read-Host