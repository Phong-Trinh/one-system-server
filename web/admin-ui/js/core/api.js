/* ─── API Client ─────────────────────────────────────────────────────────────
   Fetch wrapper for the Go backend API.
──────────────────────────────────────────────────────────────────────────── */

const API_BASE = 'http://localhost:8080/api/v1';

const api = {
    async getNodes(orgId) {
        return fetchJSON(`${API_BASE}/nodes?org_id=${orgId}`);
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
    }
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
    return res.json();
}
