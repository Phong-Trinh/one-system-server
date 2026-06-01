/* ─── Auth ───────────────────────────────────────────────────────────────────
   Login, logout, and node-context selection.
   Depends on: core/state.js, core/toast.js, core/router.js (buildSidebar, navigate).
──────────────────────────────────────────────────────────────────────────── */

/* ── Screen Switcher ─────────────────────────────────────── */
/**
 * Show exactly one top-level screen, hiding the others.
 * @param {'login-screen'|'node-selector'|'app'} id
 */
function show(id) {
  ['login-screen', 'node-selector', 'app'].forEach(screenId => {
    document.getElementById(screenId).classList.toggle('hidden', screenId !== id);
  });
}

/* ── Login ───────────────────────────────────────────────── */
function doLogin() {
  const email = document.getElementById('login-email').value.trim();
  const pass = document.getElementById('login-password').value;

  if (email === 'admin@onesystem.local' && pass === 'admin123') {
    state.loggedIn = true;
    state.currentUser = {
      email: 'admin@onesystem.local',
      name: 'System Admin',
      orgId: 'SNAPBITE_ORG',
      staffId: 'staff_hq_admin'
    };
    toast('Welcome back, Admin!', 'success');
    showNodeSelector();
  } else {
    toast('Invalid credentials. Try admin@onesystem.local / admin123', 'error');
  }
}

/* ── Logout ──────────────────────────────────────────────── */
function doLogout() {
  state.loggedIn = false;
  state.currentUser = null;
  state.node = null;
  state.page = null;
  show('login-screen');
}

/* ── Node Selector ───────────────────────────────────────── */
async function showNodeSelector() {
  state.node = null;
  show('node-selector');

  const container = document.getElementById('node-cards-container');
  container.innerHTML = '<p class="dim">Loading nodes...</p>';

  try {
    const nodes = await api.getNodes(state.currentUser.orgId);
    if (!nodes || nodes.length === 0) {
      container.innerHTML = '<p class="dim">No nodes found for this organization.</p>';
      return;
    }

    const styles = {
      'HQ': { icon: '🏢', css: 'hq', desc: 'Procurement, approvals, BOM/SOP, financials, B2B sales' },
      'FACTORY': { icon: '🏭', css: 'fac', desc: 'Production orders, KDS, machine batches, internal transfers' },
      'STORE': { icon: '🏪', css: 'sto', desc: 'POS orders, inventory, receive goods, requisitions' }
    };

    container.innerHTML = nodes.map(n => {
      const s = styles[n.type] || { icon: '📍', css: 'sto', desc: 'Node operations' };
      return `
        <div class="node-select-card ${s.css}" onclick="enterNode('${n.id}', '${n.type}', '${n.name}')">
          <div class="nsc-glow ${s.css}"></div>
          <div class="nsc-icon">${s.icon}</div>
          <div>
            <div class="nsc-type" style="color:var(--${s.css})">${n.type}</div>
            <div class="nsc-name">${n.name}</div>
          </div>
          <div class="nsc-desc">${s.desc}</div>
          <div class="nsc-arrow">→</div>
        </div>
      `;
    }).join('');
  } catch (e) {
    container.innerHTML = `<p class="error">Failed to load nodes: ${e.message}</p>`;
  }
}

/* ── Enter Node Context ──────────────────────────────────── */
/**
 * Enter a node context: build the sidebar and navigate to the first page.
 * @param {string} nodeId
 * @param {'HQ'|'FACTORY'|'STORE'} nodeType
 * @param {string} nodeName
 */
function enterNode(nodeId, nodeType, nodeName) {
  state.node = nodeId; // Set state.node to the dynamic DB ID (e.g. CUA_HANG_01)
  state.nodeType = nodeType; // Needed for routing
  state.nodeName = nodeName;
  show('app');

  // Update header and sidebar colors
  const typeMap = {
    'HQ': { css: 'hq' },
    'FACTORY': { css: 'fac' },
    'STORE': { css: 'sto' }
  };
  const css = typeMap[nodeType] ? typeMap[nodeType].css : 'sto';

  document.getElementById('sidebar-node-bar').className = 'node-bar ' + css;
  document.getElementById('hdr-node-badge').innerHTML = `<span class="badge badge-${css}">● ${nodeType}</span>`;
  document.getElementById('hdr-node-label').textContent = nodeName;

  buildSidebar();

  // Navigate to the first page of this node's nav config
  const firstPage = NAV_CONFIG[nodeType][0].pages[0];
  navigate(firstPage.id, firstPage.label);
}
