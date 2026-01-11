const fs = require('fs');
const path = require('path');

const MODULE_DIR = path.join(__dirname, '../frontend/public/modules');
const REQUIRED_EXPORTS = ['init_with_sab', 'alloc'];
const EXPECTED_IMPORTS = [
    { module: 'env', name: 'atomic_add' },
    { module: 'env', name: 'console_log' },
    { module: 'env', name: 'console_error' }
];

async function verifyModule(filename) {
    const moduleName = filename.replace('.wasm', '');
    console.log(`\n🔍 Auditing: ${filename} (Module: ${moduleName})`);
    const filePath = path.join(MODULE_DIR, filename);
    const buffer = fs.readFileSync(filePath);
    
    try {
        const module = await WebAssembly.compile(buffer);
        const exports = WebAssembly.Module.exports(module).map(e => e.name);
        const imports = WebAssembly.Module.imports(module);

        const requiredExports = [
            `${moduleName}_init_with_sab`,
            `${moduleName}_alloc`
        ];

        let missingExports = requiredExports.filter(e => !exports.includes(e));
        
        // Backward compatibility check for generic names if prefixed are missing
        if (missingExports.length > 0) {
            const fallbackExports = ['init_with_sab', 'alloc'];
            const missingFallback = fallbackExports.filter(e => !exports.includes(e));
            if (missingFallback.length > 0) {
                console.error(`❌ Missing internal exports: ${missingExports.join(', ')} (or fallback ${fallbackExports.join(', ')})`);
                return false;
            } else {
                console.log(`✅ Using legacy generic symbols: ${fallbackExports.join(', ')}`);
            }
        } else {
            console.log(`✅ All prefixed exports found (${requiredExports.join(', ')})`);
        }

        // Check if any unexpected imports exist (optional, but good for security/purity)
        const suspiciousImports = imports.filter(i => i.module !== 'env' && i.module !== 'wasi_snapshot_preview1' && i.module !== 'wasi_unstable');
        if (suspiciousImports.length > 0) {
            console.warn(`⚠️  Suspicious imports detected:`);
            suspiciousImports.forEach(i => console.warn(`   - ${i.module}.${i.name}`));
        }

        return true;
    } catch (err) {
        console.error(`❌ Failed to parse WASM binary: ${err.message}`);
        return false;
    }
}

async function run() {
    console.log("====================================================");
    console.log("INOS Architectural Binary Inspector");
    console.log("====================================================");

    if (!fs.existsSync(MODULE_DIR)) {
        console.error(`❌ Module directory not found: ${MODULE_DIR}`);
        process.exit(1);
    }

    const files = fs.readdirSync(MODULE_DIR).filter(f => f.endsWith('.wasm'));
    
    let allPassed = true;
    for (const file of files) {
        const passed = await verifyModule(file);
        if (!passed) allPassed = false;
    }

    console.log("\n====================================================");
    if (allPassed) {
        console.log("✅ ARCHITECTURAL SYMBOLS VERIFIED");
        process.exit(0);
    } else {
        console.error("❌ ARCHITECTURAL INTEGRITY BREACHED");
        process.exit(1);
    }
}

run();
