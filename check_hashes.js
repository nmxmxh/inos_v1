
function crc32c(str) {
    const table = new Uint32Array(256);
    for (let i = 0; i < 256; i++) {
        let crc = i;
        for (let j = 0; j < 8; j++) {
            if (crc & 1) crc = (crc >>> 1) ^ 0x82F63B78;
            else crc >>>= 1;
        }
        table[i] = crc;
    }
    let crc = 0xFFFFFFFF;
    const bytes = new TextEncoder().encode(str);
    for (const byte of bytes) {
        crc = (crc >>> 8) ^ table[(crc ^ byte) & 0xFF];
    }
    return (crc ^ 0xFFFFFFFF) >>> 0;
}

function fnv1a(str) {
    let hash = 0x811C9DC5;
    const bytes = new TextEncoder().encode(str);
    for (const byte of bytes) {
        hash ^= byte;
        hash = Math.imul(hash, 0x01000193);
    }
    return hash >>> 0;
}

const ids = [
    "compute", "science", "ml", "mining", "inos_storage", "drivers",
    "gpu", "storage", "crypto", "vault"
];

const MAX_MODULES_INLINE = 64;

console.log("ID\t\tCRC32C\t\tSlot\tFNV1a\t\tFinal Slot");
console.log("-".repeat(70));

const occupied = new Set();

ids.forEach(id => {
    let c = crc32c(id);
    let slot = c % MAX_MODULES_INLINE;
    const f = fnv1a(id);
    const step = (f % (MAX_MODULES_INLINE - 2)) + 1;

    let probe = 0;
    while (occupied.has(slot)) {
        slot = (slot + step) % MAX_MODULES_INLINE;
        probe++;
        if (probe > MAX_MODULES_INLINE) {
            slot = -1; // Failed
            break;
        }
    }
    occupied.add(slot);

    console.log(`${id.padEnd(14)}\t${c.toString(16)}\t${(c % MAX_MODULES_INLINE)}\t${f.toString(16)}\t${slot}`);
});
