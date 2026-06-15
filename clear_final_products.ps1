$ErrorActionPreference = "Stop"

$nodes = @("STORE", "FACTORY")
# Only true final products that are not ingredients for something else.
# Note: ITEM_BIT_TET is used as an ingredient for ITEM_HAMBURGER_BO, so we won't clear it.
$items = @("ITEM_HAMBURGER_BO", "ITEM_HAMBURGER_GA", "ITEM_KHOAI_TAY_CHIEN")

foreach ($node in $nodes) {
    foreach ($item in $items) {
        Write-Host "Clearing stock for $item at $node..."
        $bodyInitStock = @{
            node_id = $node
            item_id = $item
            qty_bu = 0.000001
        } | ConvertTo-Json
        Invoke-RestMethod -Method Post -Uri "http://localhost:8080/api/v1/inventory/init" -Body $bodyInitStock -ContentType "application/json" | Out-Null
    }
}

Write-Host "Final products cleared successfully!"
