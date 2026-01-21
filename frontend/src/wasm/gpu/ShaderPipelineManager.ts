/// <reference types="@webgpu/types" />
/**
 * ShaderPipelineManager - Handles WebGPU pipeline caching and auto-binding
 * based on Rust-generated reflection metadata.
 */

export interface WebGpuRequest {
  method: string;
  shader: string;
  analysis?: ShaderAnalysis;
  buffers: BufferDesc[];
  workgroup: [number, number, number];
  dispatch: [number, number, number];
}

export interface ShaderAnalysis {
  meta: any;
  requirements: any;
  validation: {
    hash: string;
    signature: string;
    timestamp: number;
  };
  bindings: BindingInfo[];
}

export interface BindingInfo {
  group: number;
  binding: number;
  resource_type: string; // "buffer", "texture", "sampler"
  access: string; // "read", "write", "read_write"
}

export interface BufferDesc {
  id: string;
  data: string; // Base64 encoded for initial data
  size: number;
  usage: string; // "storage", "uniform"
  type_hint: string; // "float32", "uint32"
}

export class ShaderPipelineManager {
  private device: GPUDevice;
  private pipelineCache: Map<string, GPUComputePipeline> = new Map();
  private shaderModuleCache: Map<string, GPUShaderModule> = new Map();

  constructor(device: GPUDevice) {
    this.device = device;
  }

  /**
   * Get or create a compute pipeline for the given request
   */
  async getPipeline(request: WebGpuRequest): Promise<GPUComputePipeline> {
    const hash = request.analysis?.validation.hash || this.hashString(request.shader);

    if (this.pipelineCache.has(hash)) {
      return this.pipelineCache.get(hash)!;
    }

    const shaderModule = this.getShaderModule(request.shader, hash);

    const pipeline = await this.device.createComputePipelineAsync({
      layout: 'auto',
      compute: {
        module: shaderModule,
        entryPoint: 'main', // Default to 'main' for INOS compute shaders
      },
    });

    this.pipelineCache.set(hash, pipeline);
    return pipeline;
  }

  /**
   * Get or create a shader module
   */
  private getShaderModule(code: string, hash: string): GPUShaderModule {
    if (this.shaderModuleCache.has(hash)) {
      return this.shaderModuleCache.get(hash)!;
    }

    const module = this.device.createShaderModule({
      label: `inos-shader-${hash.substring(0, 8)}`,
      code,
    });

    this.shaderModuleCache.set(hash, module);
    return module;
  }

  /**
   * Helper to hash a string if no analysis is provided
   */
  private hashString(s: string): string {
    let hash = 0;
    for (let i = 0; i < s.length; i++) {
      hash = (hash << 5) - hash + s.charCodeAt(i);
      hash |= 0;
    }
    return `hash-${hash}`;
  }

  /**
   * Automated binding orchestration based on reflection metadata
   */
  createBindGroup(
    pipeline: GPUComputePipeline,
    request: WebGpuRequest,
    gpuBuffers: Map<string, GPUBuffer>
  ): GPUBindGroup {
    const entries: GPUBindGroupEntry[] = [];

    // Map bindings to provided buffers
    // In a production system, we'd more strictly match group/binding IDs
    // For now, we assume buffers are ordered or indexed by names
    const bindings = request.analysis?.bindings || [];
    bindings.forEach(binding => {
      const desc = request.buffers[binding.binding];
      if (!desc) return;

      const buffer = gpuBuffers.get(desc.id);

      if (buffer) {
        entries.push({
          binding: binding.binding,
          resource: {
            buffer: buffer,
          },
        });
      }
    });

    return this.device.createBindGroup({
      layout: pipeline.getBindGroupLayout(0),
      entries,
    });
  }
}
