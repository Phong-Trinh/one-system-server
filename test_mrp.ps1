$ErrorActionPreference = "Stop"

Write-Host "1. Configuring NodeItemConfig for ITEM_BANH_MI..."
$bodyBanhMi = @{
    node_id = "FACTORY"
    item_id = "ITEM_BANH_MI"
    sourcing_strategy = "EXTERNAL_PROCUREMENT"
    supplier_id = "SUP_01"
    reorder_point = 50
    safety_stock = 10
} | ConvertTo-Json
Invoke-RestMethod -Method Put -Uri "http://localhost:8080/api/v1/node-item-configs" -Body $bodyBanhMi -ContentType "application/json" | Out-Null

Write-Host "2. Configuring NodeItemConfig for ITEM_HANH_TAY..."
$bodyHanhTay = @{
    node_id = "FACTORY"
    item_id = "ITEM_HANH_TAY"
    sourcing_strategy = "EXTERNAL_PROCUREMENT"
    supplier_id = "SUP_01"
    reorder_point = 100
    safety_stock = 20
} | ConvertTo-Json
Invoke-RestMethod -Method Put -Uri "http://localhost:8080/api/v1/node-item-configs" -Body $bodyHanhTay -ContentType "application/json" | Out-Null

Write-Host "3. Ensuring stock is essentially 0 for both items..."
$bodyInitBanhMi = @{ node_id = "FACTORY"; item_id = "ITEM_BANH_MI"; qty_bu = 0.000001 } | ConvertTo-Json
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/api/v1/inventory/init" -Body $bodyInitBanhMi -ContentType "application/json" | Out-Null

$bodyInitHanhTay = @{ node_id = "FACTORY"; item_id = "ITEM_HANH_TAY"; qty_bu = 0.000001 } | ConvertTo-Json
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/api/v1/inventory/init" -Body $bodyInitHanhTay -ContentType "application/json" | Out-Null

Write-Host "4. Creating Production Order for 10 ITEM_HAMBURGER_BO..."
$bodyPO = @{
    bom_id = "BOM_HAMBURGER_BO"
    node_id = "FACTORY"
    target_qty = 10
} | ConvertTo-Json
$po = Invoke-RestMethod -Method Post -Uri "http://localhost:8080/api/v1/production/orders" -Body $bodyPO -ContentType "application/json"
Write-Host "PO Created: $($po.id) with Status: $($po.status)"

Write-Host "5. Waiting 2 seconds for background MRP process to run..."
Start-Sleep -Seconds 2

Write-Host "6. Fetching generated Purchase Orders..."
$puros = Invoke-RestMethod -Method Get -Uri "http://localhost:8080/api/v1/puros?org_id=SNAPBITE_ORG"
if ($puros -eq $null) {
    Write-Host "ERROR: No Purchase Orders were created!"
} else {
    Write-Host "SUCCESS! Generated PurOs:"
    $puros | ConvertTo-Json -Depth 5
}
