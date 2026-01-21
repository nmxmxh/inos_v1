const fs = require('fs');
const path = require('path');

const ROOT = path.resolve(__dirname, '../frontend/bridge/generated/protocols/schemas');

/**
 * Cap'n Proto TS generator (capnp-es) sometimes generates imports with ".//"
 * which ESM/Vite cannot resolve. This script replaces them with relative paths.
 * Since our schemas are always at depth 2 (e.g., p2p/v1/mesh.ts),
 * ".//" usually refers to the root of the schemas directory.
 */
function rewriteFile(filePath) {
  const text = fs.readFileSync(filePath, 'utf8');
  
  // Replace ".//" with "../../"
  // This handles imports like: from ".//system/v1/runtime.js";
  const fixRegex = /(['"])\.\/\//g;
  const fixedText = text.replace(fixRegex, '$1../../');

  if (fixedText !== text) {
    console.log(`[fix-capnp-imports] Fixed imports in: ${path.relative(ROOT, filePath)}`);
    fs.writeFileSync(filePath, fixedText);
  }
}

function walk(dir) {
  if (!fs.existsSync(dir)) return;
  const entries = fs.readdirSync(dir, { withFileTypes: true });
  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      walk(fullPath);
    } else if (entry.isFile() && (fullPath.endsWith('.ts') || fullPath.endsWith('.js'))) {
      rewriteFile(fullPath);
    }
  }
}

console.log(`[fix-capnp-imports] Scanning ${ROOT}...`);
if (fs.existsSync(ROOT)) {
  walk(ROOT);
  console.log('[fix-capnp-imports] Done.');
} else {
  console.error(`[fix-capnp-imports] Missing path: ${ROOT}`);
  process.exitCode = 1;
}
