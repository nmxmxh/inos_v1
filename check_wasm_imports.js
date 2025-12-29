const fs = require('fs');
const path = require('path');

const modulesDir = path.join(__dirname, 'frontend/public/modules');
const kernelPath = path.join(__dirname, 'frontend/public/kernel.wasm');

async function checkModule(name, filePath) {
  if (!fs.existsSync(filePath)) {
    console.log(`❌ ${name} not found`);
    return;
  }

  try {
    const buffer = fs.readFileSync(filePath);
    const mod = await WebAssembly.compile(buffer);
    const imports = WebAssembly.Module.imports(mod);
    const exports = WebAssembly.Module.exports(mod);
    
    console.log(`\n📦 ${name} (${(buffer.length/1024/1024).toFixed(2)} MB)`);
    
    // Check Memory Import
    const memImport = imports.find(i => i.kind === 'memory');
    if (memImport) {
      console.log(`  ✅ Imports memory from ${memImport.module}.${memImport.name}`);
    } else {
      console.log(`  ❌ Does NOT import memory`);
    }

    // Check Memory Export
    const memExport = exports.find(e => e.kind === 'memory');
    if (memExport) {
      console.log(`  ⚠️  Exports memory as "${memExport.name}" (Self-contained heap?)`);
    }

    // List some imports
    // console.log('  Imports:', imports.slice(0, 3).map(i => `${i.module}.${i.name}`).join(', ') + (imports.length > 3 ? '...' : ''));

  } catch (e) {
    console.error(`⚠️  Failed to parse ${name}:`, e.message);
  }
}

async function main() {
  await checkModule('kernel.wasm', kernelPath);
  
  const files = fs.readdirSync(modulesDir).filter(f => f.endsWith('.wasm'));
  for (const f of files) {
    await checkModule(f, path.join(modulesDir, f));
  }
}

main();
