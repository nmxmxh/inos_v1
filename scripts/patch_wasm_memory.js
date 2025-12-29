const fs = require('fs');
const path = require('path');

const filePath = process.argv[2];
if (!filePath) {
  console.error('Usage: node patch_wasm_memory.js <path-to-wasm>');
  process.exit(1);
}

const buffer = fs.readFileSync(filePath);

if (buffer.readUInt32LE(0) !== 0x6d736100) {
  console.error('❌ Not a valid WASM file');
  process.exit(1);
}

console.log(`🔧 Patching ${path.basename(filePath)} memory for SharedArrayBuffer...`);

function decodeLeb128(buffer, offset) {
  let value = 0;
  let shift = 0;
  let bytesRead = 0;
  while (true) {
    const byte = buffer[offset + bytesRead];
    value |= (byte & 0x7f) << shift;
    bytesRead++;
    shift += 7;
    if ((byte & 0x80) === 0) break;
  }
  return { value, bytesRead };
}

let offset = 8;
let patched = false;

while (offset < buffer.length) {
  const sectionId = buffer[offset];
  const sizeResult = decodeLeb128(buffer, offset + 1);
  const size = sizeResult.value;
  const sizeBytes = sizeResult.bytesRead;
  
  const contentStart = offset + 1 + sizeBytes;
  const contentEnd = contentStart + size;

  if (sectionId === 5) { // Memory Section
    const countResult = decodeLeb128(buffer, contentStart);
    let memOffset = contentStart + countResult.bytesRead;
    
    const currentFlags = buffer[memOffset];
    
    // IN-PLACE modification: just change the flags byte
    if ((currentFlags & 0x02) === 0) { // Not already shared
      buffer[memOffset] = 0x03; // Set to Shared | HasMax
      console.log(`  ✅ Memory flags patched: 0x${currentFlags.toString(16)} → 0x03 (Shared+HasMax)`);
      patched = true;
    } else {
      console.log(`  ℹ️  Memory already shared (flags: 0x${currentFlags.toString(16)})`);
    }
    
    break;
  }

  offset = contentEnd;
}

if (patched) {
  fs.writeFileSync(filePath, buffer);
  console.log(`✅ Saved: ${path.basename(filePath)}`);
} else if (!patched) {
  console.log(`  ℹ️  No changes needed`);
}
