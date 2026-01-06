/**
 * Registry Reader for scanning WASM module entries in SharedArrayBuffer.
 * Reads module metadata and capabilities from the INOS registry.
 */

import {
  OFFSET_MODULE_REGISTRY,
  MODULE_ENTRY_SIZE,
  MAX_MODULES_INLINE,
  OFFSET_ARENA,
} from './layout';

const CAPABILITY_ENTRY_SIZE = 36;

export interface ModuleEntry {
  id: string;
  active: boolean;
  version: string;
  capabilities: string[];
  memoryUsage: number;
}

export class RegistryReader {
  private view: DataView;
  private memory: WebAssembly.Memory;
  private sabOffset: number;

  constructor(memory: WebAssembly.Memory, sabOffset: number = 0) {
    this.memory = memory;
    this.view = new DataView(memory.buffer);
    this.sabOffset = sabOffset;
  }

  private readString(offset: number, length: number): string {
    const bytes = new Uint8Array(this.memory.buffer, offset, length);
    let end = 0;
    while (end < length && bytes[end] !== 0) end++;
    // SAB decoding fix: slice to create copy before decoding
    return new TextDecoder().decode(bytes.slice(0, end));
  }

  private readCapabilities(arenaOffset: number, count: number): string[] {
    const capabilities: string[] = [];
    if (arenaOffset === 0 || count === 0) return capabilities;

    const absoluteOffset = this.sabOffset + arenaOffset;

    if (
      arenaOffset < OFFSET_ARENA ||
      absoluteOffset + count * CAPABILITY_ENTRY_SIZE > this.memory.buffer.byteLength
    ) {
      return capabilities;
    }

    for (let i = 0; i < count; i++) {
      const entryOffset = absoluteOffset + i * CAPABILITY_ENTRY_SIZE;
      const id = this.readString(entryOffset, 32);
      if (id) capabilities.push(id);
    }

    return capabilities;
  }

  scan(): Record<string, ModuleEntry> {
    const modules: Record<string, ModuleEntry> = {};

    for (let i = 0; i < MAX_MODULES_INLINE; i++) {
      const offset = this.sabOffset + OFFSET_MODULE_REGISTRY + i * MODULE_ENTRY_SIZE;
      const idHash = this.view.getUint32(offset + 8, true);

      if (idHash === 0) continue;

      const flags = this.view.getUint8(offset + 15);
      const isActive = (flags & 0b0010) !== 0;

      if (!isActive) continue;

      const moduleId = this.readString(offset + 64, 12);
      const capTableOffset = this.view.getUint32(offset + 56, true);
      const capCount = this.view.getUint16(offset + 60, true);
      const capabilities = this.readCapabilities(capTableOffset, capCount);

      modules[moduleId] = {
        id: moduleId,
        active: isActive,
        version: `${this.view.getUint8(offset + 12)}.${this.view.getUint8(offset + 13)}.${this.view.getUint8(offset + 14)}`,
        capabilities,
        memoryUsage: this.view.getUint16(offset + 34, true),
      };
    }

    return modules;
  }
}
