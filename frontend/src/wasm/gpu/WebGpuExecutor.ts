/// <reference types="@webgpu/types" />
/**
 * WebGpuExecutor - Actually executes the work on the GPU
 */
import { ShaderPipelineManager, WebGpuRequest } from './ShaderPipelineManager';

export class WebGpuExecutor {
  private device: GPUDevice | null = null;
  private manager: ShaderPipelineManager | null = null;

  async initialize(): Promise<boolean> {
    if (!navigator.gpu) {
      console.error('WebGPU not supported');
      return false;
    }

    const adapter = await navigator.gpu.requestAdapter();
    if (!adapter) return false;

    this.device = await adapter.requestDevice();
    this.manager = new ShaderPipelineManager(this.device);
    return true;
  }

  async execute(request: WebGpuRequest): Promise<Uint8Array> {
    if (!this.device || !this.manager) {
      if (!(await this.initialize())) {
        throw new Error('WebGPU failed to initialize');
      }
    }

    const device = this.device!;
    const manager = this.manager!;

    // 1. Setup Pipeline
    const pipeline = await manager.getPipeline(request);

    // 2. Prepare Buffers
    const gpuBuffers = new Map<string, GPUBuffer>();
    for (const desc of request.buffers) {
      const buffer = device.createBuffer({
        size: desc.size,
        usage: GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_SRC | GPUBufferUsage.COPY_DST,
        mappedAtCreation: desc.data.length > 0,
      });

      if (desc.data.length > 0) {
        const arrayBuffer = buffer.getMappedRange();
        const srcData = Uint8Array.from(atob(desc.data), c => c.charCodeAt(0));
        new Uint8Array(arrayBuffer).set(srcData);
        buffer.unmap();
      }

      gpuBuffers.set(desc.id, buffer);
    }

    // 3. Command Encoding
    const commandEncoder = device.createCommandEncoder();
    const passEncoder = commandEncoder.beginComputePass();
    passEncoder.setPipeline(pipeline);

    // Use reflection-aware binding
    const bindGroup = manager.createBindGroup(
      pipeline,
      request.analysis?.bindings || [],
      gpuBuffers
    );
    passEncoder.setBindGroup(0, bindGroup);

    passEncoder.dispatchWorkgroups(request.dispatch[0], request.dispatch[1], request.dispatch[2]);
    passEncoder.end();

    // 4. Read back output
    const outputBuffer = gpuBuffers.get('output');
    if (!outputBuffer) throw new Error('No output buffer defined');

    const readBuffer = device.createBuffer({
      size: outputBuffer.size,
      usage: GPUBufferUsage.COPY_DST | GPUBufferUsage.MAP_READ,
    });

    commandEncoder.copyBufferToBuffer(outputBuffer, 0, readBuffer, 0, outputBuffer.size);
    device.queue.submit([commandEncoder.finish()]);

    // 5. Map and return
    await readBuffer.mapAsync(GPUMapMode.READ);
    const result = new Uint8Array(readBuffer.getMappedRange().slice(0));
    readBuffer.unmap();

    return result;
  }
}
