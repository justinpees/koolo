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



# --- FIND NEWEST FILE ---
Write-Host "`nSearching for newest file containing: supervisor-log-"

$autoFile = Get-ChildItem -Path $tailDir -File |
Where-Object { $_.Name -like "*supervisor-log-*" } |
Sort-Object LastWriteTime -Descending |
Select-Object -First 1
	
if ($autoFile) {
    Write-Host "Auto-selected file:"
    Write-Host "  $($autoFile.Name)"
    $fileToTail = $autoFile.FullName
}
else {
    Write-Host "No matching log file found!" -ForegroundColor Red
    exit
}



Write-Host "`nTailing file:"
Write-Host "  $fileToTail"
Write-Host "-----------------------------------------`n"

# --- EXECUTE TAIL WITH CONDITIONAL COLORS ---
Get-Content -Path $fileToTail -Tail 50 -Wait | ForEach-Object {
    $line = $_
    if ($line -match "level=INFO") {
        # Orange text for INFO lines
        Write-Host $line -ForegroundColor Green
    }
    elseif ($line -match "level=DEBUG") {
        Write-Host $line -ForegroundColor Green
    }
    elseif ($line -match "level=WARN") {
        Write-Host $line -ForegroundColor Magenta
    }
    elseif ($line -match "level=ERROR") {
        Write-Host $line -ForegroundColor Red
    }
    else {
        # Default color for anything else
        Write-Host $line -ForegroundColor Green
    }
}

Write-Host "`nScript finished. Press ENTER to close."
Read-Host