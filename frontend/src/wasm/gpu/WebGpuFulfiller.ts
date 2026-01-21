import { WebGpuExecutor } from './WebGpuExecutor';
import { WebGpuRequest } from './ShaderPipelineManager';

export class WebGpuFulfiller {
  private executor: WebGpuExecutor;

  constructor() {
    this.executor = new WebGpuExecutor();
  }

  /**
   * Check if a result payload is a WebGpuRequest
   */
  public isWebGpuRequest(data: Uint8Array): boolean {
    if (data.length < 50) return false;
    // Check if it looks like a JSON object starting with {
    if (data[0] !== 123) return false;

    try {
      const text = new TextDecoder().decode(data);
      if (text.includes('"method"') && text.includes('"shader"') && text.includes('"buffers"')) {
        return true;
      }
    } catch {
      return false;
    }
    return false;
  }

  /**
   * Fulfill a WebGpuRequest and return the raw output data
   */
  public async fulfill(data: Uint8Array, sab?: SharedArrayBuffer): Promise<Uint8Array | null> {
    try {
      const text = new TextDecoder().decode(data);
      const request = JSON.parse(text) as WebGpuRequest;
      console.log(
        `[WebGpuFulfiller] ⚡ Fulfilling GPU request: ${request.method} (${request.buffers.length} buffers)`
      );

      const result = await this.executor.execute(request, sab);
      if (result) {
        console.log(`[WebGpuFulfiller] ✓ GPU Execution Success: ${result.length} bytes`);
        return result;
      }
      return null;
    } catch (err) {
      console.error('[WebGpuFulfiller] 💥 Fulfillment Failed:', err);
      return null;
    }
  }
}
