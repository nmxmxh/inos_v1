#!/usr/bin/env node

const fs = require('fs');
const path = require('path');

/**
 * Diagnostic script to track SharedArrayBuffer writes and identify corruption
 * 
 * This script:
 * 1. Inspects WASM modules for memory imports/exports
 * 2. Tracks capability table writes
 * 3. Identifies potential memory overlaps
 * 4. Verifies SAB write operations
 */

const MODULES = ['compute', 'science', 'ml', 'mining', 'vault', 'drivers'];
const MODULE_DIR = path.join(__dirname, 'frontend/public/modules');

// Known offsets from logs
const KNOWN_WRITES = {
  compute: {
    capTable: 0x150000,
    capTableSize: 36, // 1 capability * 36 bytes
    registryEntry: 0x2e0
  },
  science: {
    capTable: 0x150024,
    capTableSize: 108, // 3 capabilities * 36 bytes
    registryEntry: 0x14e0
  }
};

function readLEB128(buffer, offset) {
  let result = 0;
  let shift = 0;
  let bytesRead = 0;
  let byte;
  
  do {
    byte = buffer[offset + bytesRead];
    result |= (byte & 0x7f) << shift;
    shift += 7;
    bytesRead++;
  } while (byte & 0x80);
  
  return { value: result, bytesRead };
}

function inspectWasmModule(moduleName) {
  const wasmPath = path.join(MODULE_DIR, `${moduleName}.wasm`);
  
  if (!fs.existsSync(wasmPath)) {
    console.log(`\n❌ ${moduleName}.wasm not found`);
    return null;
  }
  
  const buffer = fs.readFileSync(wasmPath);
  
  // Verify WASM magic
  if (buffer.readUInt32LE(0) !== 0x6d736100) {
    console.log(`❌ ${moduleName}.wasm is not a valid WASM file`);
    return null;
  }
  
  console.log(`\n📦 ${moduleName}.wasm (${(buffer.length / 1024).toFixed(2)} KB)`);
  
  const info = {
    name: moduleName,
    size: buffer.length,
    hasMemoryImport: false,
    hasMemoryExport: false,
    imports: [],
    exports: [],
    dataSegments: []
  };
  
  let offset = 8; // Skip magic + version
  
  while (offset < buffer.length) {
    const sectionId = buffer[offset];
    const sizeInfo = readLEB128(buffer, offset + 1);
    const contentStart = offset + 1 + sizeInfo.bytesRead;
    const contentEnd = contentStart + sizeInfo.value;
    
    switch (sectionId) {
      case 2: // Import Section
        const importCount = readLEB128(buffer, contentStart).value;
        console.log(`  📥 Imports: ${importCount}`);
        // Parse imports to find memory imports
        let importOffset = contentStart + readLEB128(buffer, contentStart).bytesRead;
        for (let i = 0; i < importCount; i++) {
          const modLen = readLEB128(buffer, importOffset);
          const modName = buffer.toString('utf8', importOffset + modLen.bytesRead, importOffset + modLen.bytesRead + modLen.value);
          importOffset += modLen.bytesRead + modLen.value;
          
          const fieldLen = readLEB128(buffer, importOffset);
          const fieldName = buffer.toString('utf8', importOffset + fieldLen.bytesRead, importOffset + fieldLen.bytesRead + fieldLen.value);
          importOffset += fieldLen.bytesRead + fieldLen.value;
          
          const kind = buffer[importOffset++];
          
          if (kind === 0x02) { // Memory import
            info.hasMemoryImport = true;
            const flags = buffer[importOffset++];
            const isShared = (flags & 0x02) !== 0;
            console.log(`    ✓ Memory import: ${modName}.${fieldName} (shared: ${isShared})`);
          }
          
          info.imports.push({ module: modName, field: fieldName, kind });
        }
        break;
        
      case 5: // Memory Section
        console.log(`  💾 Memory Section found`);
        const memCount = buffer[contentStart];
        if (memCount > 0) {
          const flags = buffer[contentStart + 1];
          const isShared = (flags & 0x02) !== 0;
          console.log(`    Shared: ${isShared ? 'YES ✓' : 'NO ✗'}`);
        }
        break;
        
      case 7: // Export Section
        const exportCount = readLEB128(buffer, contentStart).value;
        let exportOffset = contentStart + readLEB128(buffer, contentStart).bytesRead;
        for (let i = 0; i < exportCount; i++) {
          const nameLen = readLEB128(buffer, exportOffset);
          const name = buffer.toString('utf8', exportOffset + nameLen.bytesRead, exportOffset + nameLen.bytesRead + nameLen.value);
          exportOffset += nameLen.bytesRead + nameLen.value;
          
          const kind = buffer[exportOffset++];
          const index = readLEB128(buffer, exportOffset);
          exportOffset += index.bytesRead;
          
          if (kind === 0x02) { // Memory export
            info.hasMemoryExport = true;
          }
          
          if (name.includes('init')) {
            console.log(`    ✓ Init function: ${name}`);
          }
        }
        break;
        
      case 11: // Data Section
        const dataCount = readLEB128(buffer, contentStart).value;
        console.log(`  📊 Data segments: ${dataCount}`);
        info.dataSegments.push({ count: dataCount });
        break;
    }
    
    offset = contentEnd;
  }
  
  return info;
}

function analyzeMemoryOverlaps() {
  console.log('\n\n🔍 MEMORY OVERLAP ANALYSIS\n' + '='.repeat(50));
  
  const regions = [];
  
  // Add known capability table regions
  for (const [module, data] of Object.entries(KNOWN_WRITES)) {
    regions.push({
      module,
      type: 'capability_table',
      start: data.capTable,
      end: data.capTable + data.capTableSize,
      size: data.capTableSize
    });
    
    regions.push({
      module,
      type: 'registry_entry',
      start: data.registryEntry,
      end: data.registryEntry + 96, // MODULE_ENTRY_SIZE
      size: 96
    });
  }
  
  // Sort by start address
  regions.sort((a, b) => a.start - b.start);
  
  console.log('\nMemory Regions (sorted by address):');
  regions.forEach(r => {
    console.log(`  0x${r.start.toString(16).padStart(6, '0')} - 0x${r.end.toString(16).padStart(6, '0')} (${r.size.toString().padStart(4)} bytes) ${r.module.padEnd(10)} ${r.type}`);
  });
  
  // Check for overlaps
  console.log('\nChecking for overlaps...');
  let hasOverlap = false;
  for (let i = 0; i < regions.length - 1; i++) {
    const current = regions[i];
    const next = regions[i + 1];
    
    if (current.end > next.start) {
      console.log(`  ❌ OVERLAP DETECTED!`);
      console.log(`     ${current.module} ${current.type} (0x${current.start.toString(16)} - 0x${current.end.toString(16)})`);
      console.log(`     ${next.module} ${next.type} (0x${next.start.toString(16)} - 0x${next.end.toString(16)})`);
      console.log(`     Overlap: ${current.end - next.start} bytes`);
      hasOverlap = true;
    }
  }
  
  if (!hasOverlap) {
    console.log('  ✓ No overlaps detected in known regions');
  }
}

function checkCorruptionPattern() {
  console.log('\n\n🔬 CORRUPTION PATTERN ANALYSIS\n' + '='.repeat(50));
  
  console.log('\nObserved corruption:');
  console.log('  • First 4 bytes of each capability table overwritten with 0x00000000');
  console.log('  • Affects: compute (0x150000), science (0x150024)');
  console.log('  • Pattern: Exactly 4 bytes (u32 size)');
  console.log('  • Timing: After write, before read');
  
  console.log('\nPossible causes:');
  console.log('  1. ✓ MOST LIKELY: Arena allocator writing metadata at table start');
  console.log('  2. Module initialization writing zeros to clear memory');
  console.log('  3. Race condition between write and read');
  console.log('  4. Incorrect offset calculation in write_capability_table');
  
  console.log('\nRecommended fixes:');
  console.log('  1. Add 4-byte padding before capability entries');
  console.log('  2. Write capability table with 4-byte offset');
  console.log('  3. Investigate allocate_arena for metadata writes');
  console.log('  4. Add logging to track all SAB writes to 0x150000-0x150100 range');
}

// Main execution
console.log('🔍 INOS SAB Write Corruption Diagnostic');
console.log('='.repeat(50));

// Inspect all modules
const moduleInfo = {};
for (const module of MODULES) {
  const info = inspectWasmModule(module);
  if (info) {
    moduleInfo[module] = info;
  }
}

// Analyze memory layout
analyzeMemoryOverlaps();

// Analyze corruption pattern
checkCorruptionPattern();

console.log('\n\n📋 SUMMARY\n' + '='.repeat(50));
console.log(`Modules inspected: ${Object.keys(moduleInfo).length}/${MODULES.length}`);
console.log(`\nNext steps:`);
console.log(`  1. Add logging to allocate_arena in sdk/src/registry.rs`);
console.log(`  2. Verify write order in module init functions`);
console.log(`  3. Check if arena allocator writes metadata to allocated regions`);
console.log(`  4. Test with 4-byte padding workaround`);
