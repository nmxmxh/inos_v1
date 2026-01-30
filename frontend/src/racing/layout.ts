export const SAB_OFFSETS = {
  // Arena Region (Starts at 0x150000)
  DRONE_CONTROL: 0x160000, // 32 * 16 bytes = 512 bytes
  DRONE_STATE_A: 0x160200, // 32 * 128 bytes = 4096 bytes
  DRONE_STATE_B: 0x161200, // 32 * 128 bytes = 4096 bytes
  DRONE_MATRIX_A: 0x162200, // 32 * 64 bytes = 2048 bytes
  DRONE_MATRIX_B: 0x162a00, // 32 * 64 bytes = 2048 bytes
};

export const EPOCH_INDICES = {
  DRONE_SENSOR: 48,
  DRONE_CONTROL: 49,
  DRONE_PHYSICS: 50,
  RACE_STATE: 51,
};

export const DRONE_CONSTANTS = {
  MAX_DRONES: 32,
  STRIDE: 128, // bytes
  CONTROL_STRIDE: 16, // bytes
};
