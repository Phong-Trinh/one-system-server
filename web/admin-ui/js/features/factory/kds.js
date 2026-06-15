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
      bHtml = mBatches.map(b => {
        const item = itemMap[b.item_id] || { name: b.item_id };
        const isProgress = b.status === 'IN_PROGRESS';
        let statusStyle = isProgress ? 'background: rgba(16, 185, 129, 0.05); border-color: rgba(16, 185, 129, 0.4)' : 
                          b.status === 'ALLOCATED' ? 'background: rgba(59, 130, 246, 0.05); border-color: rgba(59, 130, 246, 0.4)' : '';
        
        let timerHtml = '';
        if (isProgress) {
          const timeLeft = Math.max(0, b.duration - b.elapsed);
          const progressPercent = Math.min(100, (b.elapsed / b.duration) * 100);
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

        return `
          <div style="margin-bottom:16px; padding:12px; border:1px solid var(--border); border-radius:8px; ${statusStyle}">
            <div style="font-weight:600; margin-bottom: 4px" class="flex row justify-between align-center">
              <span>${item.name}</span>
              <span class="badge ${isProgress ? 'badge-green' : 'badge-primary'}" style="font-size:10px">${b.status}</span>
            </div>
            <div class="dim small" style="margin-bottom:12px">Qty: ${b.qty} (Slots: ${b.slots_used}) • Step: ${b.step_name}</div>
            ${timerHtml}
            ${b.status === 'ALLOCATED' ? `
              <button class="btn btn-primary fw btn-sm" onclick="startBatch('${b.id}')">▶ Start Batch</button>
            ` : ''}
            ${b.status === 'IN_PROGRESS' ? `
              <button class="btn btn-green fw btn-sm" onclick="completeBatch('${b.id}')">✔ Complete</button>
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

async function startBatch(id) {
  try {
    await api.confirmBatchPlacement(id);
    toast('Batch started!', 'success');
    navigate('fac-kds', 'Kitchen Display System');
  } catch(e) { toast(e.message, 'error'); }
}

async function completeBatch(id) {
  try {
    await api.confirmBatchCompletion(id);
    toast('Batch completed!', 'success');
    navigate('fac-kds', 'Kitchen Display System');
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
