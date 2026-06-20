/* ─── Factory / KDS ──────────────────────────────────────────────────────────
   Kitchen Display System. View machine status, batches, report breakdown.
   Wired to live API.
──────────────────────────────────────────────────────────────────────────── */

async function renderFacKDS() {
  const nodeId = state.node;
  let machines = [], equipmentTypes = [], batches = [], items = [], pool = {};
  let error = null;

  try {
    [machines, equipmentTypes, batches, items, pool] = await Promise.all([
      api.getMachines(nodeId),
      api.getEquipmentTypes(),
      api.getKDSBatches(nodeId),
      api.getItems(state.orgId),
      api.getKDSPool()
    ]);
    machines = machines || [];
    equipmentTypes = equipmentTypes || [];
    batches = batches || [];
    items = items || [];
    pool = pool || {};
  } catch(e) { error = e.message; }

  const eqTypeMap = {};
  equipmentTypes.forEach(e => eqTypeMap[e.id] = e);
  
  const itemMap = {};
  items.forEach(it => itemMap[it.id] = it);

  // Group batches by machine
  const batchesByMachine = {};
  const queuedBatches = [];
  batches.forEach(b => {
    if (b.status === 'QUEUED' && !b.machine_id) {
      queuedBatches.push(b);
    }
    if (b.status === 'COMPLETED') return; // Do not clutter machines with completed batches
    if (b.machine_id) {
      if (!batchesByMachine[b.machine_id]) batchesByMachine[b.machine_id] = [];
      batchesByMachine[b.machine_id].push(b);
    }
  });

  // Render pool
  const poolForNode = pool[nodeId] || [];
  let poolHtml = '';
  if (poolForNode.length > 0) {
    const rows = poolForNode.map(p => `
      <div class="flex row justify-between align-center" style="padding:8px 0;border-bottom:1px solid var(--border)">
        <div>
          <div style="font-weight:600">PO: <code style="font-size:11px">${p.po_id.slice(0,8)}</code></div>
          <div class="dim small">Waiting in queue...</div>
        </div>
        <div class="badge badge-orange" style="font-family:monospace">
          Auto-flush in ${p.seconds_until_flush}s
        </div>
      </div>
    `).join('');
    
    poolHtml = `
      <div class="card" style="margin-bottom:24px;border-left:4px solid var(--orange)">
        <div style="font-weight:700;margin-bottom:8px">Orchestrator Queue (${poolForNode.length} Orders waiting)</div>
        <div class="dim small" style="margin-bottom:12px">These orders are batching up and will be dispatched to stations shortly.</div>
        ${rows}
      </div>
    `;
  }

  let queuedHtml = '';
  if (queuedBatches.length > 0) {
    const qRows = queuedBatches.map(b => {
      const item = itemMap[b.item_id] || { name: b.item_id };
      return `
        <div class="flex row justify-between align-center" style="padding:8px 0;border-bottom:1px solid var(--border)">
          <div>
            <div style="font-weight:600">${item.name} <span class="dim small">— Qty: ${b.qty}</span></div>
            <div class="dim small">Step: ${b.step_name}</div>
          </div>
          <div class="badge badge-dim">WAITING FOR MACHINE</div>
        </div>
      `;
    }).join('');
    queuedHtml = `
      <div class="card" style="margin-bottom:24px;border-left:4px solid var(--border-heavy)">
        <div style="font-weight:700;margin-bottom:8px">Pending Allocation (${queuedBatches.length} batches)</div>
        <div class="dim small" style="margin-bottom:12px">These batches are decomposed but waiting for an available machine.</div>
        ${qRows}
      </div>
    `;
  }

  // Render machines
  const machineCards = machines.map(m => {
    const eqType = eqTypeMap[m.equipment_type_id] || { name: m.equipment_type_id };
    const mBatches = batchesByMachine[m.id] || [];
    
    // Sort so IN_PROGRESS shows up at the top if mixed
    mBatches.sort((a,b) => {
        if (a.status === 'IN_PROGRESS' && b.status !== 'IN_PROGRESS') return -1;
        if (b.status === 'IN_PROGRESS' && a.status !== 'IN_PROGRESS') return 1;
        return 0;
    });

    const totalSlotsUsed = mBatches.reduce((sum, b) => sum + b.slots_used, 0);
    const capacityPercent = m.max_capacity > 0 ? Math.min(100, Math.round((totalSlotsUsed / m.max_capacity) * 100)) : 0;
    const capacityColor = capacityPercent >= 100 ? 'var(--red)' : capacityPercent > 80 ? 'var(--orange)' : 'var(--green)';

    let headerColor = m.status === 'UNDER_MAINTENANCE' ? 'var(--red)' : 
                      m.status === 'BUSY' ? 'var(--primary)' : 'var(--border-heavy)';
    
    let bHtml = `
      <div style="border: 2px dashed var(--border); border-radius: 8px; padding: 24px; text-align: center; color: var(--text-dim); background: rgba(0,0,0,0.02)">
        <div style="font-size: 24px; margin-bottom: 8px">📥</div>
        <div style="font-weight: 500">Waiting for batches...</div>
      </div>
    `;
    
    if (mBatches.length > 0) {
      // Group batches by step_name + status
      const groupedBatches = {};
      mBatches.forEach(b => {
        const key = b.step_name + '_' + b.status;
        if (!groupedBatches[key]) {
          groupedBatches[key] = {
            ids: [],
            item_ids: new Set(),
            status: b.status,
            step_name: b.step_name,
            qty: 0,
            slots_used: 0,
            duration: b.duration,
            elapsed: b.elapsed,
            orders: []
          };
        }
        groupedBatches[key].ids.push(b.id);
        groupedBatches[key].item_ids.add(b.item_id);
        groupedBatches[key].qty += b.qty;
        groupedBatches[key].slots_used += b.slots_used;
        // Keep the max elapsed time if IN_PROGRESS (so timer is somewhat accurate for grouped items)
        if (b.elapsed > groupedBatches[key].elapsed) {
          groupedBatches[key].elapsed = b.elapsed;
        }
        if (b.reference_order_id && !groupedBatches[key].orders.includes(b.reference_order_id)) {
          groupedBatches[key].orders.push(b.reference_order_id);
        }
      });

      bHtml = Object.values(groupedBatches).map(gb => {
        const itemNames = Array.from(gb.item_ids).map(id => itemMap[id] ? itemMap[id].name : id).join(', ');
        const isProgress = gb.status === 'IN_PROGRESS';
        let statusStyle = isProgress ? 'background: rgba(16, 185, 129, 0.05); border-color: rgba(16, 185, 129, 0.4)' : 
                          gb.status === 'ALLOCATED' ? 'background: rgba(59, 130, 246, 0.05); border-color: rgba(59, 130, 246, 0.4)' : '';
        
        let timerHtml = '';
        if (isProgress) {
          const timeLeft = Math.max(0, gb.duration - gb.elapsed);
          const progressPercent = Math.min(100, (gb.elapsed / gb.duration) * 100);
          timerHtml = `
            <div style="margin-bottom:12px">
              <div class="flex row justify-between small" style="color: var(--green); margin-bottom: 4px; font-weight: 600">
                <span>Cooking...</span>
                <span>${timeLeft}s left</span>
              </div>
              <div style="width:100%; height:6px; background:rgba(16, 185, 129, 0.2); border-radius:3px; overflow:hidden">
                <div style="width:${progressPercent}%; height:100%; background:var(--green); transition: width 1s linear"></div>
              </div>
            </div>
          `;
        }

        const ordersHtml = gb.orders.length > 0 
          ? `<div class="dim small" style="margin-bottom:4px; font-size:11px">Orders: ${gb.orders.map(o => o.slice(0, 8)).join(', ')}</div>`
          : '';

        const idsJson = JSON.stringify(gb.ids).replace(/"/g, '&quot;');

        return `
          <div style="margin-bottom:16px; padding:12px; border:1px solid var(--border); border-radius:8px; ${statusStyle}">
            <div style="font-weight:600; margin-bottom: 4px" class="flex row justify-between align-center">
              <span>${gb.step_name}</span>
              <span class="badge ${isProgress ? 'badge-green' : 'badge-primary'}" style="font-size:10px">${gb.status}</span>
            </div>
            ${ordersHtml}
            <div class="dim small" style="margin-bottom:12px">Qty: ${gb.qty} (Slots: ${gb.slots_used}) • Items: ${itemNames}</div>
            ${timerHtml}
            ${gb.status === 'ALLOCATED' ? `
              <button class="btn btn-primary fw btn-sm" onclick="startBatches('${idsJson}')">▶ Start Batch</button>
            ` : ''}
            ${gb.status === 'IN_PROGRESS' ? `
              <button class="btn btn-green fw btn-sm" onclick="completeBatches('${idsJson}')">✔ Complete</button>
            ` : ''}
          </div>
        `;
      }).join('');
    }

    return `
      <div class="machine-card" style="border-top-color: ${headerColor}">
        <div class="mc-header">
          <div class="flex row justify-between align-start">
            <div>
              <div class="mc-id">${m.id}</div>
              <div class="mc-type">${eqType.name}</div>
            </div>
            <div class="mc-status badge ${
              m.status === 'BUSY' ? 'badge-primary' :
              m.status === 'IDLE' ? 'badge-dim' : 'badge-red'
            }">${m.status}</div>
          </div>
          <div style="margin-top: 12px; margin-bottom: 4px;">
            <div class="flex row justify-between dim small" style="margin-bottom: 4px">
              <span>Capacity Used</span>
              <span>${totalSlotsUsed} / ${m.max_capacity}</span>
            </div>
            <div style="width:100%; height:6px; background:var(--border); border-radius:3px; overflow:hidden">
              <div style="width:${capacityPercent}%; height:100%; background:${capacityColor}; transition: width 0.3s ease"></div>
            </div>
          </div>
        </div>
        <div class="mc-body">
          ${m.status === 'UNDER_MAINTENANCE' 
            ? `<div class="faint" style="color:var(--red)">Machine is out of service.</div>` 
            : bHtml}
        </div>
        <div class="mc-footer flex row justify-between" style="border-top: 1px solid var(--border); padding-top: 12px; margin-top: auto">
           <div class="small dim">Asset: ${m.linked_asset_id ? `<code style="font-size:10px">${m.linked_asset_id.slice(0,8)}</code>` : 'None'}</div>
           <div>
             ${m.status !== 'UNDER_MAINTENANCE'
               ? `<button class="btn btn-ghost btn-sm" style="color:var(--red); padding:4px 8px" onclick="reportBreakdown('${m.id}')">Report Breakdown</button>`
               : `<button class="btn btn-outline btn-sm" style="padding:4px 8px" onclick="returnToService('${m.id}')">Return to Service</button>`
             }
           </div>
        </div>
      </div>
    `;
  }).join('');

  return `
    ${pageHeader('Kitchen Display System', 'Real-time machine status and batch execution')}
    ${error ? `<div class="empty-state" style="color:var(--red)">${error}</div>` : ''}

    ${poolHtml}
    ${queuedHtml}

    <div class="grid cols-3" style="gap:24px; align-items: stretch;">
      ${machineCards}
    </div>
  `;
}

/* ── Actions ─────────────────────────────────────────────── */

async function startBatches(idsJson) {
  try {
    const ids = JSON.parse(idsJson.replace(/&quot;/g, '"'));
    await api.bulkConfirmBatchPlacement(ids);
    toast('Batches started!', 'success');
    navigate(state.page, 'Kitchen Display System');
  } catch(e) { toast(e.message, 'error'); }
}

async function completeBatches(idsJson) {
  try {
    const ids = JSON.parse(idsJson.replace(/&quot;/g, '"'));
    await api.bulkConfirmBatchCompletion(ids);
    toast('Batches completed!', 'success');
    navigate(state.page, 'Kitchen Display System');
  } catch(e) { toast(e.message, 'error'); }
}

async function reportBreakdown(id) {
  if (!confirm('Report breakdown? This will mark the machine under maintenance.')) return;
  toast('Not fully implemented: sync with asset maintenance', 'warning');
  // Currently we would hit a machine update endpoint or asset sync status.
}

async function returnToService(id) {
  toast('Not fully implemented: sync with asset maintenance', 'warning');
}
