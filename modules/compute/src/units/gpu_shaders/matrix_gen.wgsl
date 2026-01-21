/**
 * INOS Matrix Generation Shader
 * 
 * Computes 4x4 instance matrices for bird parts (Body, Head, Beak, Wings, Tail)
 * directly from bird physics state in the SAB.
 */

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

struct Config {
    bird_count: u32,
    dt: f32,
    time: f32,
    padding: f32,
}

@group(0) @binding(0) var<storage, read> birds : array<Bird>;
@group(0) @binding(1) var<storage, read_write> bodies : array<mat4x4<f32>>;
@group(0) @binding(2) var<storage, read_write> heads : array<mat4x4<f32>>;
@group(0) @binding(3) var<storage, read_write> beaks : array<mat4x4<f32>>;
@group(0) @binding(4) var<storage, read_write> left_wings : array<mat4x4<f32>>;
@group(0) @binding(5) var<storage, read_write> left_wing_tips : array<mat4x4<f32>>;
@group(0) @binding(6) var<storage, read_write> right_wings : array<mat4x4<f32>>;
@group(0) @binding(7) var<storage, read_write> right_wing_tips : array<mat4x4<f32>>;
@group(0) @binding(8) var<storage, read_write> tails : array<mat4x4<f32>>;
@group(0) @binding(9) var<uniform> config : Config;

fn quaternion_to_matrix(q: vec4<f32>) -> mat4x4<f32> {
    let x = q.x;
    let y = q.y;
    let z = q.z;
    let w = q.w;
    
    return mat4x4<f32>(
        1.0 - 2.0*y*y - 2.0*z*z, 2.0*x*y + 2.0*w*z, 2.0*x*z - 2.0*w*y, 0.0,
        2.0*x*y - 2.0*w*z, 1.0 - 2.0*x*x - 2.0*z*z, 2.0*y*z + 2.0*w*x, 0.0,
        2.0*x*z + 2.0*w*y, 2.0*y*z - 2.0*w*x, 1.0 - 2.0*x*x - 2.0*y*y, 0.0,
        0.0, 0.0, 0.0, 1.0
    );
}

fn translation_matrix(v: vec3<f32>) -> mat4x4<f32> {
    return mat4x4<f32>(
        1.0, 0.0, 0.0, 0.0,
        0.0, 1.0, 0.0, 0.0,
        0.0, 0.0, 1.0, 0.0,
        v.x, v.y, v.z, 1.0
    );
}

fn rotation_x(angle: f32) -> mat4x4<f32> {
    let s = sin(angle);
    let c = cos(angle);
    return mat4x4<f32>(
        1.0, 0.0, 0.0, 0.0,
        0.0, c,   s,   0.0,
        0.0, -s,  c,   0.0,
        0.0, 0.0, 0.0, 1.0
    );
}

fn rotation_y(angle: f32) -> mat4x4<f32> {
    let s = sin(angle);
    let c = cos(angle);
    return mat4x4<f32>(
        c,   0.0, -s,  0.0,
        0.0, 1.0, 0.0, 0.0,
        s,   0.0, c,   0.0,
        0.0, 0.0, 0.0, 1.0
    );
}

fn rotation_z(angle: f32) -> mat4x4<f32> {
    let s = sin(angle);
    let c = cos(angle);
    return mat4x4<f32>(
        c,   s,   0.0, 0.0,
        -s,  c,   0.0, 0.0,
        0.0, 0.0, 1.0, 0.0,
        0.0, 0.0, 0.0, 1.0
    );
}

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) global_id : vec3<u32>) {
    let idx = global_id.x;
    if (idx >= config.bird_count) {
        return;
    }

    let b = birds[idx];
    let pos = vec3<f32>(b.pos_x, b.pos_y, b.pos_z);
    let rot = vec4<f32>(b.rot_x, b.rot_y, b.rot_z, b.rot_w);
    
    // Constant rotations from Rust/MathUnit
    let BODY_ROT = rotation_x(1.570796); // FRAC_PI_2
    let BEAK_ROT = rotation_x(1.570796);

    // Bird root transform
    let bird_rot = quaternion_to_matrix(rot);
    let bird_root = translation_matrix(pos) * bird_rot;

    // 0. Body
    bodies[idx] = bird_root * BODY_ROT;

    // 1. Head
    heads[idx] = bird_root * translation_matrix(vec3<f32>(0.0, 0.0, 0.18));

    // 2. Beak
    beaks[idx] = bird_root * translation_matrix(vec3<f32>(0.0, 0.0, 0.26)) * BEAK_ROT;

    // 3. Left Wing
    let lw_flap = b.wing_left;
    let lw_p1 = translation_matrix(vec3<f32>(-0.04, 0.0, 0.05)) * rotation_z(lw_flap);
    let lw_m2 = bird_root * lw_p1;
    left_wings[idx] = lw_m2 * translation_matrix(vec3<f32>(-0.15, 0.0, 0.0));

    // 4. Left Wing Tip
    let lwt_p3 = translation_matrix(vec3<f32>(-0.3, 0.0, 0.0)) * rotation_z(lw_flap * 0.5);
    left_wing_tips[idx] = lw_m2 * lwt_p3 * translation_matrix(vec3<f32>(-0.12, 0.0, -0.05));

    // 5. Right Wing
    let rw_flap = b.wing_right;
    let rw_p1 = translation_matrix(vec3<f32>(0.04, 0.0, 0.05)) * rotation_z(rw_flap);
    let rw_m2 = bird_root * rw_p1;
    right_wings[idx] = rw_m2 * translation_matrix(vec3<f32>(0.15, 0.0, 0.0));

    // 6. Right Wing Tip
    let rwt_p3 = translation_matrix(vec3<f32>(0.3, 0.0, 0.0)) * rotation_z(rw_flap * 0.5);
    right_wing_tips[idx] = rw_m2 * rwt_p3 * translation_matrix(vec3<f32>(0.12, 0.0, -0.05));

    // 7. Tail
    let tail_yaw = b.wing_tail;
    let tail_p1 = translation_matrix(vec3<f32>(0.0, 0.0, -0.15)) * rotation_y(tail_yaw);
    tails[idx] = bird_root * tail_p1 * translation_matrix(vec3<f32>(0.0, 0.0, -0.1));
}

