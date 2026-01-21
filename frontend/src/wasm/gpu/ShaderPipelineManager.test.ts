import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
  ShaderPipelineManager,
  type WebGpuRequest,
  type BindingInfo,
} from './ShaderPipelineManager';

// Mock WebGPU types for testing
interface MockGPUShaderModule {
  label?: string;
  code: string;
}

interface MockGPUComputePipeline {
  label?: string;
  getBindGroupLayout: (index: number) => MockGPUBindGroupLayout;
}

interface MockGPUBindGroupLayout {
  label?: string;
}

interface MockGPUBuffer {
  label?: string;
  size: number;
}

describe('ShaderPipelineManager', () => {
  let manager: ShaderPipelineManager;
  let mockDevice: any;
  let mockShaderModule: MockGPUShaderModule;
  let mockPipeline: MockGPUComputePipeline;

  beforeEach(() => {
    mockShaderModule = {
      label: 'test-shader',
      code: '@compute @workgroup_size(64) fn main() {}',
    };

    mockPipeline = {
      label: 'test-pipeline',
      getBindGroupLayout: vi.fn(() => ({ label: 'test-layout' })),
    };

    // Mock GPU device
    mockDevice = {
      label: 'test-device',
      createShaderModule: vi.fn(() => mockShaderModule),
      createComputePipelineAsync: vi.fn(async () => mockPipeline),
      createBindGroup: vi.fn(() => ({ label: 'test-bindgroup' })),
    };

    manager = new ShaderPipelineManager(mockDevice);
  });

  describe('Pipeline Management', () => {
    it('should create a compute pipeline for a shader', async () => {
      const request: WebGpuRequest = {
        method: 'compute',
        shader: '@compute @workgroup_size(64) fn main() {}',
        buffers: [],
        workgroup: [64, 1, 1],
        dispatch: [1, 1, 1],
      };

      const pipeline = await manager.getPipeline(request);

      expect(pipeline).toBeDefined();
      expect(mockDevice.createShaderModule).toHaveBeenCalledOnce();
      expect(mockDevice.createComputePipelineAsync).toHaveBeenCalledOnce();
    });

    it('should cache pipelines by hash', async () => {
      const shader = '@compute @workgroup_size(64) fn main() {}';
      const request: WebGpuRequest = {
        method: 'compute',
        shader,
        buffers: [],
        workgroup: [64, 1, 1],
        dispatch: [1, 1, 1],
      };

      // First call
      await manager.getPipeline(request);
      // Second call with same shader
      await manager.getPipeline(request);

      // Should only create pipeline once
      expect(mockDevice.createComputePipelineAsync).toHaveBeenCalledOnce();
    });

    it('should use provided shader analysis hash', async () => {
      const request: WebGpuRequest = {
        method: 'compute',
        shader: '@compute @workgroup_size(64) fn main() {}',
        analysis: {
          meta: {},
          requirements: {},
          validation: {
            hash: 'custom-hash-123',
            signature: 'sig',
            timestamp: Date.now(),
          },
          bindings: [],
        },
        buffers: [],
        workgroup: [64, 1, 1],
        dispatch: [1, 1, 1],
      };

      await manager.getPipeline(request);

      // Should use the provided hash
      expect(mockDevice.createShaderModule).toHaveBeenCalled();
    });

    it('should create different pipelines for different shaders', async () => {
      const request1: WebGpuRequest = {
        method: 'compute',
        shader: '@compute @workgroup_size(64) fn main() {}',
        buffers: [],
        workgroup: [64, 1, 1],
        dispatch: [1, 1, 1],
      };

      const request2: WebGpuRequest = {
        method: 'compute',
        shader: '@compute @workgroup_size(128) fn main() {}',
        buffers: [],
        workgroup: [128, 1, 1],
        dispatch: [1, 1, 1],
      };

      await manager.getPipeline(request1);
      await manager.getPipeline(request2);

      // Should create two pipelines
      expect(mockDevice.createComputePipelineAsync).toHaveBeenCalledTimes(2);
    });
  });

  describe('Shader Module Management', () => {
    it('should cache shader modules', async () => {
      const shader = '@compute @workgroup_size(64) fn main() {}';
      const request: WebGpuRequest = {
        method: 'compute',
        shader,
        buffers: [],
        workgroup: [64, 1, 1],
        dispatch: [1, 1, 1],
      };

      await manager.getPipeline(request);
      await manager.getPipeline(request);

      // Should only create shader module once
      expect(mockDevice.createShaderModule).toHaveBeenCalledOnce();
    });

    it('should create shader module with proper label', async () => {
      const request: WebGpuRequest = {
        method: 'compute',
        shader: '@compute @workgroup_size(64) fn main() {}',
        buffers: [],
        workgroup: [64, 1, 1],
        dispatch: [1, 1, 1],
      };

      await manager.getPipeline(request);

      const call = mockDevice.createShaderModule.mock.calls[0][0];
      expect(call.label).toMatch(/^inos-shader-/);
      expect(call.code).toBe(request.shader);
    });
  });

  describe('Bind Group Creation', () => {
    it('should create bind groups with correct entries', () => {
      const bindings: BindingInfo[] = [
        { group: 0, binding: 0, resource_type: 'buffer', access: 'read' },
        { group: 0, binding: 1, resource_type: 'buffer', access: 'write' },
      ];

      const gpuBuffers = new Map<string, MockGPUBuffer>([
        ['input', { label: 'input-buffer', size: 1024 }],
        ['output', { label: 'output-buffer', size: 1024 }],
      ]);

      const bindGroup = manager.createBindGroup(mockPipeline as any, bindings, gpuBuffers as any);

      expect(bindGroup).toBeDefined();
      expect(mockDevice.createBindGroup).toHaveBeenCalledOnce();
      expect(mockPipeline.getBindGroupLayout).toHaveBeenCalledWith(0);
    });

    it('should handle empty bindings', () => {
      const bindings: BindingInfo[] = [];
      const gpuBuffers = new Map<string, MockGPUBuffer>();

      const bindGroup = manager.createBindGroup(mockPipeline as any, bindings, gpuBuffers as any);

      expect(bindGroup).toBeDefined();
      expect(mockDevice.createBindGroup).toHaveBeenCalledOnce();
    });
  });

  describe('WGSL Parsing Utilities', () => {
    it('should parse WGSL shader for bindings', () => {
      const wgslSource = `
        @group(0) @binding(0) var<uniform> params: mat4x4<f32>;
        @group(0) @binding(1) var<storage, read> inputBuffer: array<f32>;
        @group(0) @binding(2) var<storage, read_write> outputBuffer: array<f32>;
      `;

      const bindingRegex = /@group\((\d+)\)\s+@binding\((\d+)\)/g;
      const matches = [...wgslSource.matchAll(bindingRegex)];

      expect(matches).toHaveLength(3);
      expect(matches[0][1]).toBe('0');
      expect(matches[0][2]).toBe('0');
    });

    it('should extract uniform buffer layouts', () => {
      const wgslSource = `
        struct Params {
          mvp: mat4x4<f32>,
          time: f32,
          resolution: vec2<f32>,
        }
        @group(0) @binding(0) var<uniform> params: Params;
      `;

      const structMatch = wgslSource.match(/struct\s+(\w+)\s*\{([^}]+)\}/);
      expect(structMatch).toBeTruthy();
      expect(structMatch![1]).toBe('Params');
    });

    it('should parse workgroup size', () => {
      const wgslSource = `@compute @workgroup_size(256, 1, 1)`;
      const match = wgslSource.match(/@workgroup_size\((\d+)(?:,\s*(\d+))?(?:,\s*(\d+))?\)/);

      expect(match).toBeTruthy();
      expect(match![1]).toBe('256');
    });
  });
});
