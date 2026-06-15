const http = require('http');

const API_BASE = 'http://localhost:8080/api/v1';

async function fetchJSON(url, options = {}) {
    if (options.body) {
        options.headers = { ...options.headers, 'Content-Type': 'application/json' };
    }
    const response = await fetch(`${API_BASE}${url}`, options);
    if (!response.ok) {
        const text = await response.text();
        throw new Error(`API error ${response.status} on ${url}: ${text}`);
    }
    const txt = await response.text();
    return txt ? JSON.parse(txt) : null;
}

async function run() {
    console.log("--- STARTING END-TO-END SUPPLY CHAIN INTEGRATION TEST ---");
    
    // 1. Get Node IDs
    const orgs = await fetchJSON('/orgs');
    const orgId = orgs[0].id;
    const nodes = await fetchJSON(`/nodes?org_id=${orgId}`);
    
    const storeNode = nodes.find(n => n.type === 'STORE');
    const hqNode = nodes.find(n => n.type === 'HQ');
    const factoryNode = nodes.find(n => n.type === 'FACTORY');
    
    if (!storeNode || !hqNode || !factoryNode) {
        throw new Error("Missing required nodes (HQ, FACTORY, STORE) in seed data");
    }
    
    console.log(`[Nodes] Store: ${storeNode.id}, Factory: ${factoryNode.id}, HQ: ${hqNode.id}`);
    
    // 2. Configure ROP
    console.log(`[Config] Setting ROP=20 for ITEM_BIT_TET at Store (INTERNAL_TRANSFER)...`);
    await fetchJSON('/node-item-configs', {
        method: 'PUT',
        body: JSON.stringify({
            node_id: storeNode.id,
            item_id: 'ITEM_BIT_TET',
            sourcing_strategy: 'INTERNAL_TRANSFER',
            provider_node_id: factoryNode.id,
            reorder_point: 20,
            safety_stock: 10
        })
    });

    console.log(`[Config] Setting ROP=1000 for ITEM_BO_TUOI at Factory (EXTERNAL_PROCUREMENT)...`);
    await fetchJSON('/node-item-configs', {
        method: 'PUT',
        body: JSON.stringify({
            node_id: factoryNode.id,
            item_id: 'ITEM_BO_TUOI',
            sourcing_strategy: 'EXTERNAL_PROCUREMENT',
            reorder_point: 1000,
            safety_stock: 500
        })
    });
    
    // 3. Initialize stock
    console.log(`[Stock] Initializing Store stock of ITEM_BIT_TET to 25...`);
    await fetchJSON('/inventory/init', {
        method: 'POST',
        body: JSON.stringify({ node_id: storeNode.id, item_id: 'ITEM_BIT_TET', qty_bu: 25 })
    });
    
    // Add stock for Bánh Mì and Hành Tây
    await fetchJSON('/inventory/init', {
        method: 'POST',
        body: JSON.stringify({ node_id: storeNode.id, item_id: 'ITEM_BANH_MI', qty_bu: 100 })
    });
    await fetchJSON('/inventory/init', {
        method: 'POST',
        body: JSON.stringify({ node_id: storeNode.id, item_id: 'ITEM_HANH_TAY', qty_bu: 1000 })
    });

    console.log(`[Stock] Initializing Factory stock of ITEM_BO_TUOI to 4000...`);
    await fetchJSON('/inventory/init', {
        method: 'POST',
        body: JSON.stringify({ node_id: factoryNode.id, item_id: 'ITEM_BO_TUOI', qty_bu: 4000 })
    });

    // 4. Create POS Order for 10 Hamburger Bò
    // This should backflush 10 ITEM_BIT_TET at Store, dropping stock to 15 (below ROP 20).
    console.log(`[POS] Creating POS Sale Order for 10 Hamburger Bò...`);
    const saleOrder = await fetchJSON('/orders', {
        method: 'POST',
        body: JSON.stringify({
            org_id: orgId,
            node_id: storeNode.id,
            source: 'DIRECT_POS',
            items: [
                {
                    item_id: 'ITEM_HAMBURGER_BO',
                    quantity: 10,
                    price: 5.0
                }
            ]
        })
    });
    
    console.log(`[POS] Completing Sale Order ${saleOrder.id}...`);
    await fetchJSON(`/orders/${saleOrder.id}/complete`, { method: 'PATCH' });
    
    // Wait briefly for background facade hooks
    await new Promise(r => setTimeout(r, 1000));
    
    // 5. Verify ITO was created
    console.log(`[ITO] Checking if Internal Transfer Order was generated...`);
    const itos = await fetchJSON(`/itos?node_id=${storeNode.id}`);
    const autoIto = itos.find(i => i.trigger === 'ROP_AUTOMATIC' && i.status === 'AUTO_APPROVED');
    
    if (!autoIto) {
        throw new Error("Auto ITO not found! ROP trigger or POS Backflushing failed.");
    }
    console.log(`[ITO] ✅ Auto ITO found: ${autoIto.id}`);
    
    // 6. Verify Production Order was created at Factory
    console.log(`[PO] Checking if Production Order was generated at Factory...`);
    const pos = await fetchJSON(`/production/orders?node_id=${factoryNode.id}`);
    const autoPo = pos.find(p => p.item_id === 'ITEM_BIT_TET' && p.status === 'PENDING');
    
    if (!autoPo) {
        throw new Error("Auto Production Order not found! Facade routing failed.");
    }
    console.log(`[PO] ✅ Auto Production Order found: ${autoPo.id}`);
    
    // 7. Complete Production Order via KDS flow to trigger Stock Deduction
    console.log(`[Production] Completing KDS Batches for Production Order ${autoPo.id}...`);
    // Wait for orchestrator to decompose (min_batch_window_s is 8s + tick 5s)
    await new Promise(r => setTimeout(r, 15000));
    
    let isPOCompleted = false;
    let attempts = 0;
    while (!isPOCompleted && attempts < 10) {
        attempts++;
        const batches = await fetchJSON(`/kds/batches?node_id=${factoryNode.id}`);
        const poBatches = batches.filter(b => b.po_id === autoPo.id && b.status !== 'COMPLETED');
        
        for (const batch of poBatches) {
            console.log(`[KDS] Confirming placement for batch ${batch.id} (Step: ${batch.sop_step_id})...`);
            await fetchJSON(`/kds/batches/${batch.id}/confirm-placement`, { method: 'POST' });

            console.log(`[KDS] Confirming completion for batch ${batch.id} (Step: ${batch.sop_step_id})...`);
            await fetchJSON(`/kds/batches/${batch.id}/confirm-completion`, { method: 'POST' });
        }
        
        // Check PO status
        const updatedPo = await fetchJSON(`/production/orders/${autoPo.id}`);
        if (updatedPo.status === 'COMPLETED') {
            isPOCompleted = true;
            console.log(`[PO] ✅ Production Order is now COMPLETED.`);
            break;
        }
        
        // Wait before next check
        await new Promise(r => setTimeout(r, 2000));
    }
    
    if (!isPOCompleted) {
        throw new Error("PO was never completed via KDS flow.");
    }

    // 8. Verify Factory consumed ITEM_BO_TUOI and triggered Draft PO to HQ
    console.log(`[Procurement] Checking if Draft Purchase Order was generated at HQ...`);
    const hqPOs = await fetchJSON(`/puros?delivery_node_id=${factoryNode.id}`);
    if (hqPOs && hqPOs.length > 0) {
        const draftPO = hqPOs.find(p => p.status === 'DRAFT');
        if (draftPO) {
             console.log(`[Procurement] ✅ Draft Purchase Order to HQ found: ${draftPO.id}`);
        } else {
             console.log(`[Procurement] ❌ Draft PO not found. Available POs:`, hqPOs);
             throw new Error("External Procurement PO not generated.");
        }
    } else {
        throw new Error("No Purchase Orders found at HQ!");
    }

    console.log("--- REPLENISHMENT END-TO-END TEST PASSED SUCCESSFULLY ---");
}

run().catch(err => {
    console.error("Test failed:", err);
    process.exit(1);
});
