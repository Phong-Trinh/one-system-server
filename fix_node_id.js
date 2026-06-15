const fs = require('fs');
const path = require('path');

const dir = 'c:/Users/ADMIN/Documents/OneSystem/one-system-server/web/admin-ui/js/features';
const filesToFix = [
  'store/inventory.js',
  'store/pos-orders.js',
  'store/internal-transfers.js',
  'factory/production-orders.js',
  'factory/kds.js',
  'factory/inventory.js',
  'factory/internal-transfers.js'
];

filesToFix.forEach(rel => {
  const full = path.join(dir, rel);
  if (fs.existsSync(full)) {
    let content = fs.readFileSync(full, 'utf8');
    content = content.replace(/const nodeId = state\.nodeId;/g, 'const nodeId = state.node;');
    fs.writeFileSync(full, content);
    console.log(`Fixed ${rel}`);
  }
});
