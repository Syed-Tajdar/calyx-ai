$env:Path = [System.Environment]::GetEnvironmentVariable('Path','Machine') + ';' + [System.Environment]::GetEnvironmentVariable('Path','User')
Write-Host 'Machine Go:' (([System.Environment]::GetEnvironmentVariable('Path','Machine') -split ';') | Where-Object { $_ -match 'Go|go' })
Write-Host 'User Go:' (([System.Environment]::GetEnvironmentVariable('Path','User') -split ';') | Where-Object { $_ -match 'Go|go' })
Write-Host 'where:'
where.exe go
Write-Host 'version:'
go version
