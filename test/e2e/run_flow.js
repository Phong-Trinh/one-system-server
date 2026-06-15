const http = require('http');

const API_BASE = 'http://localhost:8080/api/v1';

async function fetchJSON(url, options = {}) {
    if (options.body) {
        options.headers = { ...options.headers, 'Content-Type': 'application/json' };
    }
    const response = await fetch(`${API_BASE}${url}`, options);
    if (!response.ok) {
        const text = await response.text();
        throw new Error(`API error ${response.status}: ${text}`);
    }
    const txt = await response.text();
    return txt ? JSON.parse(txt) : null;
}

async function run() {
    console.log("--- STARTING REPLENISHMENT END-TO-END TEST ---");
    
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
    
    // 2. Configure ROP for ITEM_KHOAI_TAY_CHIEN at Store
    const itemId = 'ITEM_KHOAI_TAY_CHIEN';
    console.log(`[Config] Setting ROP=20 for ${itemId} at Store...`);
    await fetchJSON('/node-item-configs', {
        method: 'PUT',
        body: JSON.stringify({
            node_id: storeNode.id,
            item_id: itemId,
            sourcing_strategy: 'INTERNAL_TRANSFER',
            provider_node_id: factoryNode.id,
            reorder_point: 20,
            safety_stock: 10
        })
    });
    
    // 3. Initialize stock to 25 at Store
    console.log(`[Stock] Initializing Store stock to 25...`);
    await fetchJSON('/inventory/init', {
        method: 'POST',
        body: JSON.stringify({
            node_id: storeNode.id,
            item_id: itemId,
            qty_bu: 25
        })
    });

    // 4. Create POS Order for 10 units (Will drop stock to 15, triggering ROP of 20)
    console.log(`[POS] Creating POS Sale Order for 10 units...`);
    const saleOrder = await fetchJSON('/orders', {
        method: 'POST',
        body: JSON.stringify({
            org_id: orgId,
            node_id: storeNode.id,
            source: 'DIRECT_POS',
            items: [
                {
                    item_id: itemId,
                    quantity: 10,
                    price: 5.0
                }
            ]
        })
    });
    
    console.log(`[POS] Completing Sale Order ${saleOrder.id}...`);
    await fetchJSON(`/orders/${saleOrder.id}/complete`, { method: 'PATCH' });
    
    // Wait briefly for background facade hooks
    await new Promise(r => setTimeout(r, 500));
    
    // 5. Verify ITO was created
    console.log(`[ITO] Checking if Internal Transfer Order was generated...`);
    const itos = await fetchJSON(`/itos?node_id=${storeNode.id}`);
    const autoIto = itos.find(i => i.trigger === 'ROP_AUTOMATIC' && i.status === 'AUTO_APPROVED');
    
    if (!autoIto) {
        throw new Error("Auto ITO not found! ROP trigger failed.");
    }
    console.log(`[ITO] ✅ Auto ITO found: ${autoIto.id}`);
    
    // 6. Verify Production Order was created at Factory
    console.log(`[PO] Checking if Production Order was generated at Factory...`);
    const pos = await fetchJSON(`/production/orders?node_id=${factoryNode.id}`);
    const autoPo = pos.find(p => p.item_id === itemId && p.status === 'PENDING');
    
    if (!autoPo) {
        throw new Error("Auto Production Order not found! Facade routing failed.");
    }
    console.log(`[PO] ✅ Auto Production Order found: ${autoPo.id}`);
    
    // 7. Verify PO is in KDS Pool
    console.log(`[KDS] Checking KDS Pool for order ${autoPo.id}...`);
    const pool = await fetchJSON(`/kds/pool`);
    const factoryPool = pool[factoryNode.id] || [];
    const poolEntry = factoryPool.find(p => p.po_id === autoPo.id);
    
    if (!poolEntry) {
        throw new Error("PO not found in KDS Pool! Orchestrator enqueue failed.");
    }
    console.log(`[KDS] ✅ PO is in Orchestrator Pool, waiting to flush in ${poolEntry.seconds_until_flush}s`);

    console.log("--- REPLENISHMENT END-TO-END TEST PASSED SUCCESSFULLY ---");
}

run().catch(err => {
    console.error("Test failed:", err);
    process.exit(1);
});
