/* ─── API Client ─────────────────────────────────────────────────────────────
   Fetch wrapper for the Go backend API.
──────────────────────────────────────────────────────────────────────────── */

const API_BASE = 'http://localhost:8080/api/v1';

const api = {
    async getNodes(orgId) {
        return fetchJSON(`${API_BASE}/nodes?org_id=${orgId}`);
    },

    // ── Items ──
    async getItems(orgId) {
        return fetchJSON(`${API_BASE}/items${orgId ? `?org_id=${orgId}` : ''}`);
    },
    async getItem(id) {
        return fetchJSON(`${API_BASE}/items/${id}`);
    },
    async createItem(data) {
        return fetchJSON(`${API_BASE}/items`, { method: 'POST', body: JSON.stringify(data) });
    },
    async updateItem(id, data) {
        return fetchJSON(`${API_BASE}/items/${id}`, { method: 'PUT', body: JSON.stringify(data) });
    },
    async deleteItem(id) {
        return fetchJSON(`${API_BASE}/items/${id}`, { method: 'DELETE' });
    },

    // ── Purchase Requisitions ──
    async submitPR(data) {
        return fetchJSON(`${API_BASE}/prs`, { method: 'POST', body: JSON.stringify(data) });
    },
    async getPRs(nodeId) {
        return fetchJSON(`${API_BASE}/prs?node_id=${nodeId}`);
    },
    async getPendingPRs(orgId) {
        return fetchJSON(`${API_BASE}/prs?org_id=${orgId}`);
    },
    async getPR(prId) {
        return fetchJSON(`${API_BASE}/prs/${prId}`);
    },
    async approvePR(prId, staffId, note) {
        return fetchJSON(`${API_BASE}/prs/${prId}/approve`, {
            method: 'PATCH',
            body: JSON.stringify({ reviewer_staff_id: staffId, note: note })
        });
    },
    async rejectPR(prId, staffId, note) {
        return fetchJSON(`${API_BASE}/prs/${prId}/reject`, {
            method: 'PATCH',
            body: JSON.stringify({ reviewer_staff_id: staffId, note: note })
        });
    },

    // ── Purchase Orders ──
    async getPOs(orgId) {
        return fetchJSON(`${API_BASE}/puros?org_id=${orgId}`);
    },
    async getPOsByNode(nodeId) {
        return fetchJSON(`${API_BASE}/puros?delivery_node_id=${nodeId}`);
    },
    async createPO(data) {
        return fetchJSON(`${API_BASE}/puros`, { method: 'POST', body: JSON.stringify(data) });
    },
    async markPOShipped(poId) {
        return fetchJSON(`${API_BASE}/puros/${poId}/ship`, { method: 'PATCH' });
    },
    async confirmPO(poId) {
        return fetchJSON(`${API_BASE}/puros/${poId}/confirm`, { method: 'PATCH' });
    },
    async confirmDraftPO(poId, data) {
        return fetchJSON(`${API_BASE}/puros/${poId}/confirm-draft`, {
            method: 'PATCH',
            body: JSON.stringify(data)
        });
    },
    async getPO(poId) {
        return fetchJSON(`${API_BASE}/puros/${poId}`);
    },

    // ── Goods Receipts ──
    async confirmGR(data) {
        return fetchJSON(`${API_BASE}/grs`, { method: 'POST', body: JSON.stringify(data) });
    },
    async getGR(grId) {
        return fetchJSON(`${API_BASE}/grs/${grId}`);
    },

    // ── Invoices ──
    async recordInvoice(data) {
        return fetchJSON(`${API_BASE}/invoices`, { method: 'POST', body: JSON.stringify(data) });
    },
    async getInvoice(invoiceId) {
        return fetchJSON(`${API_BASE}/invoices/${invoiceId}`);
    },
    async match3Way(invoiceId, grId, staffId) {
        return fetchJSON(`${API_BASE}/invoices/${invoiceId}/3way-match`, {
            method: 'POST',
            body: JSON.stringify({ gr_id: grId, matched_by_staff_id: staffId })
        });
    },

    // ── Assets ──
    async getAssets(nodeId) {
        return fetchJSON(`${API_BASE}/assets?node_id=${nodeId}`);
    },
    async registerMachine(assetId, data) {
        return fetchJSON(`${API_BASE}/assets/${assetId}/register-machine`, {
            method: 'POST',
            body: JSON.stringify(data)
        });
    },
    async syncAssetStatus(assetId, status) {
        return fetchJSON(`${API_BASE}/assets/${assetId}/status`, {
            method: 'PATCH',
            body: JSON.stringify({ status })
        });
    },

    // ── Master Data ──
    async getSuppliers(orgId) {
        return fetchJSON(`${API_BASE}/suppliers?org_id=${orgId}`);
    },
    async createSupplier(data) {
        return fetchJSON(`${API_BASE}/suppliers`, { method: 'POST', body: JSON.stringify(data) });
    },
    async getEquipmentTypes() {
        return fetchJSON(`${API_BASE}/equipment-types`);
    },
    async getMachines(nodeId) {
        return fetchJSON(`${API_BASE}/machines${nodeId ? `?node_id=${nodeId}` : ''}`);
    },
    async getMachine(id) {
        return fetchJSON(`${API_BASE}/machines/${id}`);
    },

    // ── BOM & SOP ──
    async getBOMs() {
        return fetchJSON(`${API_BASE}/production/boms`);
    },
    async getBOMByID(id) {
        return fetchJSON(`${API_BASE}/production/boms/${id}`);
    },
    async getBOMByItem(itemId) {
        return fetchJSON(`${API_BASE}/production/boms/by-item/${itemId}`);
    },
    async createBOM(data) {
        return fetchJSON(`${API_BASE}/production/boms`, { method: 'POST', body: JSON.stringify(data) });
    },
    async updateBOM(id, data) {
        return fetchJSON(`${API_BASE}/production/boms/${id}`, { method: 'PUT', body: JSON.stringify(data) });
    },
    async getSOPs() {
        return fetchJSON(`${API_BASE}/production/sops`);
    },
    async getSOPByBOM(bomId) {
        return fetchJSON(`${API_BASE}/production/sops/by-bom/${bomId}`);
    },
    async createSOP(data) {
        return fetchJSON(`${API_BASE}/production/sops`, { method: 'POST', body: JSON.stringify(data) });
    },
    async updateSOP(id, data) {
        return fetchJSON(`${API_BASE}/production/sops/${id}`, { method: 'PUT', body: JSON.stringify(data) });
    },

    // ── Production Orders ──
    async getProductionOrders(nodeId) {
        return fetchJSON(`${API_BASE}/production/orders${nodeId ? `?node_id=${nodeId}` : ''}`);
    },
    async getProductionOrder(id) {
        return fetchJSON(`${API_BASE}/production/orders/${id}`);
    },
    async createProductionOrder(data) {
        return fetchJSON(`${API_BASE}/production/orders`, { method: 'POST', body: JSON.stringify(data) });
    },
    async updateProductionOrderStatus(id, status, actualOutput) {
        return fetchJSON(`${API_BASE}/production/orders/${id}/status`, {
            method: 'PATCH',
            body: JSON.stringify({ status, actual_output: actualOutput })
        });
    },

    // ── KDS ──
    async getKDSBatches(nodeId, status) {
        const params = new URLSearchParams();
        if (nodeId) params.set('node_id', nodeId);
        if (status) params.set('status', status);
        return fetchJSON(`${API_BASE}/kds/batches?${params}`);
    },
    async getKDSPool() {
        return fetchJSON(`${API_BASE}/kds/pool`);
    },
    async confirmBatchPlacement(batchId) {
        return fetchJSON(`${API_BASE}/kds/batches/${batchId}/confirm-placement`, { method: 'POST' });
    },
    async confirmBatchCompletion(batchId, data) {
        return fetchJSON(`${API_BASE}/kds/batches/${batchId}/confirm-completion`, {
            method: 'POST',
            body: JSON.stringify(data || {})
        });
    },

    // ── Internal Transfer Orders (ITO) ──
    async getITOs(nodeId) {
        return fetchJSON(`${API_BASE}/itos?node_id=${nodeId}`);
    },
    async getITO(id) {
        return fetchJSON(`${API_BASE}/itos/${id}`);
    },
    async createITO(data) {
        return fetchJSON(`${API_BASE}/itos`, { method: 'POST', body: JSON.stringify(data) });
    },
    async approveITO(id) {
        return fetchJSON(`${API_BASE}/itos/${id}/approve`, { method: 'PATCH' });
    },
    async rejectITO(id, reason) {
        return fetchJSON(`${API_BASE}/itos/${id}/reject`, {
            method: 'PATCH',
            body: JSON.stringify({ reason })
        });
    },
    async itoGoodsIssue(id, data) {
        return fetchJSON(`${API_BASE}/itos/${id}/goods-issue`, {
            method: 'POST',
            body: JSON.stringify(data)
        });
    },
    async itoGoodsReceipt(id, data) {
        return fetchJSON(`${API_BASE}/itos/${id}/goods-receipt`, {
            method: 'POST',
            body: JSON.stringify(data)
        });
    },

    // ── Sale Orders ──
    async getSaleOrders(nodeId) {
        return fetchJSON(`${API_BASE}/orders?node_id=${nodeId}`);
    },
    async getSaleOrder(id) {
        return fetchJSON(`${API_BASE}/orders/${id}`);
    },
    async createSaleOrder(data) {
        return fetchJSON(`${API_BASE}/orders`, { method: 'POST', body: JSON.stringify(data) });
    },
    async completeSaleOrder(id) {
        return fetchJSON(`${API_BASE}/orders/${id}/complete`, { method: 'PATCH' });
    },
    async cancelSaleOrder(id) {
        return fetchJSON(`${API_BASE}/orders/${id}/cancel`, { method: 'PATCH' });
    },

    // ── Inventory ──
    async getInventory(nodeId) {
        return fetchJSON(`${API_BASE}/inventory?node_id=${nodeId}`);
    },
    async initStock(nodeId, itemId, qtyBU) {
        return fetchJSON(`${API_BASE}/inventory/init`, {
            method: 'POST',
            body: JSON.stringify({ node_id: nodeId, item_id: itemId, qty_bu: qtyBU })
        });
    },

    // ── Node Item Config (ROP) ──
    async getNodeItemConfigs(nodeId) {
        return fetchJSON(`${API_BASE}/node-item-configs?node_id=${nodeId}`);
    },
    async upsertNodeItemConfig(data) {
        return fetchJSON(`${API_BASE}/node-item-configs`, {
            method: 'PUT',
            body: JSON.stringify(data)
        });
    },
};

async function fetchJSON(url, options = {}) {
    if (!options.headers) {
        options.headers = { 'Content-Type': 'application/json' };
    }
    const res = await fetch(url, options);
    if (!res.ok) {
        let msg = res.statusText;
        try {
            const err = await res.json();
            msg = err.error || msg;
        } catch (e) { }
        throw new Error(msg);
    }
    // 204 No Content
    if (res.status === 204) return null;
    return res.json();
}
