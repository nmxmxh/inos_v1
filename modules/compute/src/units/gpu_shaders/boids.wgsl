struct Bird {
    pos_x: f32,
    pos_y: f32,
    pos_z: f32,
    vel_x: f32,
    vel_y: f32,
    vel_z: f32,
    rot_x: f32,
    rot_y: f32,
    rot_z: f32,
    rot_w: f32,
    energy: f32,
    wing_left: f32,
    wing_right: f32,
    wing_tail: f32,
    fitness: f32,
    weights: array<f32, 44>,
}

@group(0) @binding(0) var<storage, read> input: array<Bird>;
@group(0) @binding(1) var<storage, read_write> output: array<Bird>;
@group(0) @binding(2) var<uniform> config: Config;

struct Config {
    bird_count: u32,
    dt: f32,
    time: f32,
    padding: f32,
}

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) global_id: vec3<u32>) {
    let idx = global_id.x;
    if (idx >= arrayLength(&input)) {
        return;
    }

    var boid = input[idx];
    
    // Simple motion for test verification
    boid.pos_x += boid.vel_x * 0.1;
    boid.pos_y += boid.vel_y * 0.1;
    boid.pos_z += boid.vel_z * 0.1;
    
    // Boundary bounce
    let speed_sq = boid.pos_x * boid.pos_x + boid.pos_y * boid.pos_y + boid.pos_z * boid.pos_z;
    if (speed_sq > 1600.0) { // 40.0 squared
        boid.vel_x *= -1.0;
        boid.vel_y *= -1.0;
        boid.vel_z *= -1.0;
    }

    output[idx] = boid;
}



