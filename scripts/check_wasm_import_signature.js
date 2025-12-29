const fs = require('fs');
const path = require('path');

const modulePath = process.argv[2] || 'frontend/public/modules/compute.wasm';

function checkImportSignature(filePath) {
  if (!fs.existsSync(filePath)) {
    console.log(`❌ ${filePath} not found`);
    return;
  }

  const buffer = fs.readFileSync(filePath);
  if (buffer.readUInt32LE(0) !== 0x6d736100) return;

  console.log(`📦 Inspecting Import Signature: ${path.basename(filePath)}`);

  let offset = 8;
  while (offset < buffer.length) {
    const sectionId = buffer[offset];
    let size = 0;
    let sizeBytes = 0;
    
    let shift = 0;
    let byte = 0;
    let tempOffset = offset + 1;
    while (true) {
      byte = buffer[tempOffset];
      size |= (byte & 0x7f) << shift;
      sizeBytes++;
      shift += 7;
      tempOffset++;
      if ((byte & 0x80) === 0) break;
    }

    const contentStart = offset + 1 + sizeBytes;

    if (sectionId === 2) { // Import Section
       let currentPos = contentStart;
       // Read count
       let count = 0;
       shift = 0;
       while (true) {
         byte = buffer[currentPos++];
         count |= (byte & 0x7f) << shift;
         shift += 7;
         if ((byte & 0x80) === 0) break;
       }

       for (let i = 0; i < count; i++) {
         // Read Module Name
         let len = 0;
         shift = 0;
         while (true) {
            byte = buffer[currentPos++];
            len |= (byte & 0x7f) << shift;
            shift += 7;
            if ((byte & 0x80) === 0) break;
         }
         const moduleName = buffer.slice(currentPos, currentPos + len).toString();
         currentPos += len;

         // Read Field Name
         len = 0;
         shift = 0;
         while (true) {
            byte = buffer[currentPos++];
            len |= (byte & 0x7f) << shift;
            shift += 7;
            if ((byte & 0x80) === 0) break;
         }
         const fieldName = buffer.slice(currentPos, currentPos + len).toString();
         currentPos += len;

         const kind = buffer[currentPos++];
         
         if (kind === 2) { // Memory Import
             const flags = buffer[currentPos++];
             let initial = 0;
             shift = 0;
             while (true) {
               byte = buffer[currentPos++];
               initial |= (byte & 0x7f) << shift;
               shift += 7;
               if ((byte & 0x80) === 0) break;
             }
             
             let maximum = -1;
             if (flags & 0x01) { // Has Max
                 maximum = 0;
                 shift = 0;
                 while (true) {
                   byte = buffer[currentPos++];
                   maximum |= (byte & 0x7f) << shift;
                   shift += 7;
                   if ((byte & 0x80) === 0) break;
                 }
             }

             console.log(`  🔍 Import: ${moduleName}.${fieldName}`);
             console.log(`     - Flags: 0x${flags.toString(16)} (Shared: ${(flags & 0x02) ? 'YES' : 'NO'})`);
             console.log(`     - Initial: ${initial}`);
             console.log(`     - Maximum: ${maximum !== -1 ? maximum : 'NONE'}`);
         } else if (kind === 0) { // Function
             // Read type index (LEB128)
             while ((buffer[currentPos++] & 0x80) !== 0);
         } else if (kind === 1) { // Table
             currentPos++; // type
             let flags = buffer[currentPos++];
             // skip initial
             while ((buffer[currentPos++] & 0x80) !== 0);
             if (flags & 1) {
                // skip max
                while ((buffer[currentPos++] & 0x80) !== 0);
             }
         } else if (kind === 3) { // Global
             currentPos++; // type
             currentPos++; // mutability
         }
       }
       return;
    }
    offset = contentStart + size;
  }
}

checkImportSignature(modulePath);
