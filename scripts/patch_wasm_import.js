const fs = require('fs');
const path = require('path');

const filePath = process.argv[2];
if (!filePath) {
  console.error('Usage: node patch_wasm_import.js <path-to-wasm>');
  process.exit(1);
}

const buffer = fs.readFileSync(filePath);

if (buffer.readUInt32LE(0) !== 0x6d736100) {
  console.error('❌ Not a valid WASM file');
  process.exit(1);
}

console.log(`🔧 Patching ${path.basename(filePath)} imports for SharedArrayBuffer...`);

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

  if (sectionId === 2) { // Import Section
    let pos = contentStart;
    const countResult = decodeLeb128(buffer, pos);
    const count = countResult.value;
    pos += countResult.bytesRead;

    for (let i = 0; i < count; i++) {
      // Module name
      const modNameLenResult = decodeLeb128(buffer, pos);
      pos += modNameLenResult.bytesRead;
      const modName = buffer.slice(pos, pos + modNameLenResult.value).toString();
      pos += modNameLenResult.value;

      // Field name
      const fieldNameLenResult = decodeLeb128(buffer, pos);
      pos += fieldNameLenResult.bytesRead;
      const fieldName = buffer.slice(pos, pos + fieldNameLenResult.value).toString();
      pos += fieldNameLenResult.value;

      // Kind
      const kind = buffer[pos++];

      if (kind === 2 && modName === 'env' && fieldName === 'memory') {
        // Memory import - IN-PLACE patch
        const currentFlags = buffer[pos];
        
        if ((currentFlags & 0x02) === 0) { // Not already shared
          buffer[pos] = 0x03; // Set to Shared | HasMax
          console.log(`  ✅ Patched env.memory flags: 0x${currentFlags.toString(16)} → 0x03 (Shared+HasMax)`);
          patched = true;
        } else {
          console.log(`  ℹ️  env.memory already shared (flags: 0x${currentFlags.toString(16)})`);
        }
        break;
      } else {
        // Skip import descriptor
        if (kind === 0) { // Function
          const typeIdxResult = decodeLeb128(buffer, pos);
          pos += typeIdxResult.bytesRead;
        } else if (kind === 1) { // Table
          pos++; // elem type
          const limitsFlags = decodeLeb128(buffer, pos);
          pos += limitsFlags.bytesRead;
          const minResult = decodeLeb128(buffer, pos);
          pos += minResult.bytesRead;
          if (limitsFlags.value & 0x01) {
            const maxResult = decodeLeb128(buffer, pos);
            pos += maxResult.bytesRead;
          }
        } else if (kind === 2) { // Memory
          const limitsFlags = decodeLeb128(buffer, pos);
          pos += limitsFlags.bytesRead;
          const minResult = decodeLeb128(buffer, pos);
          pos += minResult.bytesRead;
          if (limitsFlags.value & 0x01) {
            const maxResult = decodeLeb128(buffer, pos);
            pos += maxResult.bytesRead;
          }
        } else if (kind === 3) { // Global
          pos += 2; // valtype + mut
        }
      }
    }
    break;
  }

  offset = contentEnd;
}

if (patched) {
  fs.writeFileSync(filePath, buffer);
  console.log(`✅ Saved: ${path.basename(filePath)}`);
} else {
  console.log(`  ℹ️  No env.memory import found or already patched`);
}
