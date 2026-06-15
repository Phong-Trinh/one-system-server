$ErrorActionPreference = "Stop"

$items = @("ITEM_BANH_MI", "ITEM_HANH_TAY", "ITEM_BO_TUOI", "ITEM_GA_TUOI", "ITEM_KHOAI_TAY")

foreach ($item in $items) {
    Write-Host "Ensuring stock is 1000 for $item at FACTORY..."
    $bodyInitStock = @{
        node_id = "FACTORY"
        item_id = $item
        qty_bu = 1000
    } | ConvertTo-Json
    Invoke-RestMethod -Method Post -Uri "http://localhost:8080/api/v1/inventory/init" -Body $bodyInitStock -ContentType "application/json" | Out-Null
}

Write-Host "Factory raw materials initialized to 1000 successfully!"
