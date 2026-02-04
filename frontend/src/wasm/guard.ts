import {
  IDX_INBOX_DIRTY,
  IDX_OUTBOX_HOST_DIRTY,
  IDX_OUTBOX_KERNEL_DIRTY,
  IDX_MESH_EVENT_EPOCH,
  IDX_ARENA_ALLOCATOR,
  OFFSET_REGION_GUARDS,
  SIZE_REGION_GUARDS,
  REGION_GUARD_ENTRY_SIZE,
  REGION_GUARD_COUNT,
} from './layout';

export enum RegionOwner {
  Kernel = 1 << 0,
  Module = 1 << 1,
  Host = 1 << 2,
  System = 1 << 3,
}

export enum RegionId {
  Inbox = 0,
  OutboxHost = 1,
  OutboxKernel = 2,
  MeshEventQueue = 3,
  ArenaRequestQueue = 4,
  ArenaResponseQueue = 5,
  Arena = 6,
}

export enum AccessMode {
  ReadOnly = 0,
  SingleWriter = 1,
  MultiWriter = 2,
}

export type RegionPolicy = {
  regionId: RegionId;
  access: AccessMode;
  writerMask: number;
  readerMask: number;
  epochIndex?: number;
};

const GUARD_LOCK = 0;
const GUARD_LAST_EPOCH = 1;
const GUARD_VIOLATIONS = 2;
const GUARD_LAST_OWNER = 3;

export function policyFor(region: RegionId): RegionPolicy {
  switch (region) {
    case RegionId.Inbox:
      return {
        regionId: region,
        access: AccessMode.SingleWriter,
        writerMask: RegionOwner.Kernel,
        readerMask: RegionOwner.Module | RegionOwner.Host,
        epochIndex: IDX_INBOX_DIRTY,
      };
    case RegionId.OutboxHost:
      return {
        regionId: region,
        access: AccessMode.SingleWriter,
        writerMask: RegionOwner.Kernel,
        readerMask: RegionOwner.Host,
        epochIndex: IDX_OUTBOX_HOST_DIRTY,
      };
    case RegionId.OutboxKernel:
      return {
        regionId: region,
        access: AccessMode.MultiWriter,
        writerMask: RegionOwner.Module,
        readerMask: RegionOwner.Kernel,
        epochIndex: IDX_OUTBOX_KERNEL_DIRTY,
      };
    case RegionId.MeshEventQueue:
      return {
        regionId: region,
        access: AccessMode.SingleWriter,
        writerMask: RegionOwner.Kernel,
        readerMask: RegionOwner.Host,
        epochIndex: IDX_MESH_EVENT_EPOCH,
      };
    case RegionId.ArenaRequestQueue:
      return {
        regionId: region,
        access: AccessMode.SingleWriter,
        writerMask: RegionOwner.Module,
        readerMask: RegionOwner.Kernel,
        epochIndex: IDX_ARENA_ALLOCATOR,
      };
    case RegionId.ArenaResponseQueue:
      return {
        regionId: region,
        access: AccessMode.SingleWriter,
        writerMask: RegionOwner.Kernel,
        readerMask: RegionOwner.Module,
      };
    case RegionId.Arena:
      return {
        regionId: region,
        access: AccessMode.MultiWriter,
        writerMask: RegionOwner.Kernel | RegionOwner.Module,
        readerMask: RegionOwner.Kernel | RegionOwner.Module | RegionOwner.Host,
      };
    default:
      return {
        regionId: region,
        access: AccessMode.ReadOnly,
        writerMask: 0,
        readerMask: 0,
      };
  }
}

function guardIndex(regionId: number, field: number): number {
  const entryWords = REGION_GUARD_ENTRY_SIZE / 4;
  return regionId * entryWords + field;
}

function getGuardView(sab: SharedArrayBuffer): Int32Array {
  return new Int32Array(sab, OFFSET_REGION_GUARDS, SIZE_REGION_GUARDS / 4);
}

export function resetGuardTable(sab: SharedArrayBuffer): void {
  const view = getGuardView(sab);
  view.fill(0);
}

function incrementViolation(view: Int32Array, regionId: number): void {
  Atomics.add(view, guardIndex(regionId, GUARD_VIOLATIONS), 1);
}

export function validateRegionRead(
  sab: SharedArrayBuffer,
  region: RegionId,
  owner: RegionOwner
): boolean {
  const policy = policyFor(region);
  if ((policy.readerMask & owner) === 0) {
    const view = getGuardView(sab);
    incrementViolation(view, region);
    return false;
  }
  return true;
}

export class RegionGuard {
  private guardView: Int32Array;
  private flagsView: Int32Array | null;
  private policy: RegionPolicy;
  private owner: RegionOwner;
  private startEpoch?: number;
  private locked: boolean;

  constructor(
    guardView: Int32Array,
    flagsView: Int32Array | null,
    policy: RegionPolicy,
    owner: RegionOwner,
    locked: boolean
  ) {
    this.guardView = guardView;
    this.flagsView = flagsView;
    this.policy = policy;
    this.owner = owner;
    this.locked = locked;
    if (policy.epochIndex !== undefined && this.flagsView) {
      this.startEpoch = Atomics.load(this.flagsView, policy.epochIndex);
    }
  }

  ensureEpochAdvanced(): boolean {
    if (
      this.policy.epochIndex === undefined ||
      this.startEpoch === undefined ||
      !this.flagsView
    ) {
      return true;
    }
    const current = Atomics.load(this.flagsView, this.policy.epochIndex);
    if (current <= this.startEpoch) {
      incrementViolation(this.guardView, this.policy.regionId);
      return false;
    }
    Atomics.store(this.guardView, guardIndex(this.policy.regionId, GUARD_LAST_EPOCH), current);
    return true;
  }

  release(): boolean {
    if (!this.locked) {
      return true;
    }
    const idx = guardIndex(this.policy.regionId, GUARD_LOCK);
    const released =
      Atomics.compareExchange(this.guardView, idx, this.owner, 0) === this.owner;
    if (!released) {
      incrementViolation(this.guardView, this.policy.regionId);
    }
    this.locked = false;
    return released;
  }
}

export function acquireRegionWrite(
  sab: SharedArrayBuffer,
  flagsView: Int32Array | null,
  region: RegionId,
  owner: RegionOwner
): RegionGuard | null {
  const policy = policyFor(region);
  if ((policy.writerMask & owner) === 0) {
    const view = getGuardView(sab);
    incrementViolation(view, region);
    return null;
  }

  const view = getGuardView(sab);
  let locked = false;
  if (policy.access === AccessMode.SingleWriter) {
    const idx = guardIndex(region, GUARD_LOCK);
    locked = Atomics.compareExchange(view, idx, 0, owner) === 0;
    if (!locked) {
      incrementViolation(view, region);
      return null;
    }
  } else if (policy.access === AccessMode.MultiWriter) {
    Atomics.store(view, guardIndex(region, GUARD_LAST_OWNER), owner);
  }

  return new RegionGuard(view, flagsView, policy, owner, locked);
}
