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
    console.log("--- STARTING UI TEST SETUP ---");
    
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
    
    // 2 & 3. Configure ROP and initialize stock for all products
    const items = ['ITEM_HAMBURGER_BO', 'ITEM_HAMBURGER_GA', 'ITEM_BIT_TET', 'ITEM_KHOAI_TAY_CHIEN'];
    
    for (const itemId of items) {
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
        
        console.log(`[Stock] Initializing Store stock to 25 for ${itemId}...`);
        await fetchJSON('/inventory/init', {
            method: 'POST',
            body: JSON.stringify({
                node_id: storeNode.id,
                item_id: itemId,
                qty_bu: 25
            })
        });
    }
    console.log("--- UI TEST SETUP COMPLETED ---");
}

run().catch(err => {
    console.error("Test failed:", err);
    process.exit(1);
});
