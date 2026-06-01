/* ─── HQ / BOM & SOP ─────────────────────────────────────────────────────────
   Bill of Materials viewer and editor (HQ-only authority).
   Depends on: helpers.js, mock-data.js, toast.js.
──────────────────────────────────────────────────────────────────────────── */

function renderHQBOM() {
  return `
    ${pageHeader('BOM & SOP', 'Bills of Materials and Standard Operating Procedures — managed at HQ only')}
    <div class="grid-2 gap-16">

      ${BOMS.map(b => `
        <div class="card">
          <div class="flex ai-c jc-sb gap-8" style="margin-bottom:12px">
            <div>
              <h3>${b.output}</h3>
              <span class="small dim">${b.id} · v${b.version}</span>
            </div>
            <span class="badge badge-purple">BOM</span>
          </div>

          <hr style="margin-bottom:12px" />
          <h4 style="margin-bottom:8px;color:var(--text-dim)">Components</h4>

          ${b.lines.map(l => `
            <div class="flex ai-c jc-sb gap-8"
                 style="padding:6px 0; border-bottom:1px solid var(--border)">
              <span>${l.item}</span>
              <span class="badge badge-dim">${l.qty} ${l.unit}</span>
            </div>
          `).join('')}

          <button class="btn btn-ghost btn-sm fw mt-16"
                  style="margin-top:16px; width:100%"
                  onclick="toast('BOM editor coming in v2', 'info')">
            ✏️ Edit BOM
          </button>
        </div>
      `).join('')}

      <!-- New BOM placeholder -->
      <div class="card"
           style="display:flex; align-items:center; justify-content:center;
                  border-style:dashed; min-height:180px; cursor:pointer"
           onclick="toast('BOM creator coming in v2', 'info')">
        <div style="text-align:center; color:var(--text-faint)">
          <div style="font-size:32px; margin-bottom:8px">＋</div>
          <div>New BOM</div>
        </div>
      </div>

    </div>
  `;
}
