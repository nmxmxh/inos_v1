const fs = require('fs');
const path = require('path');

const kernelPath = path.join(__dirname, 'frontend/public/kernel.wasm');

function checkMemorySection(filePath) {
  if (!fs.existsSync(filePath)) {
    console.log(`❌ ${filePath} not found`);
    return;
  }

  const buffer = fs.readFileSync(filePath);
  
  // WASM Magic + Version
  if (buffer.readUInt32LE(0) !== 0x6d736100) {
    console.log('❌ Not a valid WASM file');
    return;
  }

  console.log(`📦 Inspecting ${path.basename(filePath)} (${(buffer.length/1024/1024).toFixed(2)} MB)`);

  let offset = 8;
  while (offset < buffer.length) {
    const sectionId = buffer[offset];
    let size = 0;
    let sizeBytes = 0;
    
    // Read LEB128 size
    let shift = 0;
    let byte = 0;
    while (true) {
      byte = buffer[offset + 1 + sizeBytes];
      size |= (byte & 0x7f) << shift;
      sizeBytes++;
      shift += 7;
      if ((byte & 0x80) === 0) break;
    }

    const contentStart = offset + 1 + sizeBytes;

    if (sectionId === 5) { // Memory Section
      console.log('  ✅ Found Memory Section (ID: 5)');
      const count = buffer[contentStart]; // Assuming count is small (1 byte LEB128)
      if (count !== 1) {
        console.log(`  ⚠️  Unexpected memory count: ${count}`);
      } else {
        const flags = buffer[contentStart + 1];
        const shared = (flags & 0x02) !== 0;
        const hasMax = (flags & 0x01) !== 0; // Flag 0x01 indicates explicit maximum
        
        console.log(`  📊 Flags: 0x${flags.toString(16)}`);
        console.log(`     - Has Maximum: ${hasMax ? 'YES' : 'NO'}`);
        console.log(`     - Is Shared:   ${shared ? 'YES' : 'NO'}`);
        
        if (!shared) {
             console.log('  ❌ FAILURE: Memory is NOT marked as shared.');
        } else {
             console.log('  ✅ SUCCESS: Memory IS marked as shared.');
        }
      }
      return; 
    }

    offset = contentStart + size;
  }
  console.log('  ❌ No Memory Section found (Imported?)');
}

checkMemorySection(kernelPath);
checkMemorySection(path.join(__dirname, 'frontend/public/modules/compute.wasm'));
