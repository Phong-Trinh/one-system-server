$ErrorActionPreference = "Stop"

$items = @("ITEM_BANH_MI", "ITEM_HANH_TAY", "ITEM_BO_TUOI", "ITEM_GA_TUOI", "ITEM_KHOAI_TAY", "ITEM_BIT_TET")

foreach ($item in $items) {
    Write-Host "Configuring NodeItemConfig for $item at STORE..."
    $bodyConfig = @{
        node_id = "STORE"
        item_id = $item
        sourcing_strategy = "INTERNAL_TRANSFER"
        provider_node_id = "FACTORY"
        reorder_point = 20
        safety_stock = 10
    } | ConvertTo-Json
    Invoke-RestMethod -Method Put -Uri "http://localhost:8080/api/v1/node-item-configs" -Body $bodyConfig -ContentType "application/json" | Out-Null

    Write-Host "Ensuring stock is 100 for $item at STORE..."
    $bodyInitStock = @{
        node_id = "STORE"
        item_id = $item
        qty_bu = 100
    } | ConvertTo-Json
    Invoke-RestMethod -Method Post -Uri "http://localhost:8080/api/v1/inventory/init" -Body $bodyInitStock -ContentType "application/json" | Out-Null
}

Write-Host "Store ingredients configured successfully!"
