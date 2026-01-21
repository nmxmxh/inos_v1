/// <reference types="@webgpu/types" />
/**
 * WebGpuExecutor - Actually executes the work on the GPU
 */
import { ShaderPipelineManager, WebGpuRequest } from './ShaderPipelineManager';

export class WebGpuExecutor {
  private device: GPUDevice | null = null;
  private manager: ShaderPipelineManager | null = null;
  private bufferCache: Map<string, GPUBuffer> = new Map();

  async initialize(): Promise<boolean> {
    if (!navigator.gpu) return false;
    const adapter = await navigator.gpu.requestAdapter();
    if (!adapter) return false;
    this.device = await adapter.requestDevice();
    this.manager = new ShaderPipelineManager(this.device);
    return true;
  }

  async execute(request: WebGpuRequest, sab?: SharedArrayBuffer): Promise<Uint8Array | null> {
    if (!this.device || !this.manager) {
      if (!(await this.initialize())) throw new Error('WebGPU Init Failed');
    }

    const device = this.device!;
    const manager = this.manager!;

    // 1. Get/Create Pipeline
    const pipeline = await manager.getPipeline(request);

    // 2. Prepare/Update Buffers (Caching strategy)
    for (const desc of request.buffers) {
      let buffer = this.bufferCache.get(desc.id);

      if (!buffer || buffer.size < desc.size) {
        buffer = device.createBuffer({
          size: desc.size,
          usage:
            GPUBufferUsage.STORAGE |
            GPUBufferUsage.UNIFORM |
            GPUBufferUsage.COPY_SRC |
            GPUBufferUsage.COPY_DST,
        });
        this.bufferCache.set(desc.id, buffer);
      }

      // Fast Path: Update from SAB or data
      if (sab && (desc.id === 'input' || desc.id === 'birds')) {
        const offset = (request as any).birdsOffset || 0;
        const view = new Uint8Array(sab, offset, desc.size);
        // @ts-ignore - SharedArrayBuffer support in WebGPU
        device.queue.writeBuffer(buffer, 0, view);
      } else if (desc.data && desc.data.length > 0) {
        const srcData = Uint8Array.from(atob(desc.data), c => c.charCodeAt(0));
        device.queue.writeBuffer(buffer, 0, srcData);
      }
    }

    // 3. Command Encoding
    const commandEncoder = device.createCommandEncoder();
    const passEncoder = commandEncoder.beginComputePass();
    passEncoder.setPipeline(pipeline);

    // Simple binding for resident buffers
    const bindGroup = manager.createBindGroup(pipeline, request, this.bufferCache);

    passEncoder.setBindGroup(0, bindGroup);

    passEncoder.dispatchWorkgroups(request.dispatch[0], request.dispatch[1], request.dispatch[2]);
    passEncoder.end();

    // 4. Output Handling (Collect all requested output buffers)
    // In INOS, output buffers have empty data in the request.
    const outputDescs = request.buffers.filter(
      b => b.id.startsWith('matrix_') || b.id === 'output'
    );
    console.log(`[WebGpuExecutor] Output buffers found: ${outputDescs.map(b => b.id).join(', ')}`);

    if (outputDescs.length === 0) {
      device.queue.submit([commandEncoder.finish()]);
      return null;
    }

    const readBuffers: { id: string; buffer: GPUBuffer; size: number }[] = [];
    for (const desc of outputDescs) {
      const gpuBuffer = this.bufferCache.get(desc.id)!;
      const readBuffer = device.createBuffer({
        size: desc.size,
        usage: GPUBufferUsage.COPY_DST | GPUBufferUsage.MAP_READ,
      });
      commandEncoder.copyBufferToBuffer(gpuBuffer, 0, readBuffer, 0, desc.size);
      readBuffers.push({ id: desc.id, buffer: readBuffer, size: desc.size });
    }

    device.queue.submit([commandEncoder.finish()]);

    // Map and collect results
    const results: Uint8Array[] = [];
    for (const rb of readBuffers) {
      await rb.buffer.mapAsync(GPUMapMode.READ);
      results.push(new Uint8Array(rb.buffer.getMappedRange().slice(0)));
      rb.buffer.unmap();
      rb.buffer.destroy();
    }

    // Combine results
    const totalSize = results.reduce((acc, r) => acc + r.length, 0);
    const combined = new Uint8Array(totalSize);
    let offset = 0;
    for (const res of results) {
      combined.set(res, offset);
      offset += res.length;
    }

    return combined;
  }
}
