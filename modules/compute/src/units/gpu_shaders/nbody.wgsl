// N-Body Gravitational Simulation
// Optimized version with tile-based force calculation and double buffering

struct Particle {
    position: vec3<f32>,
    velocity: vec3<f32>,
    mass: f32,
}

struct SimParams {
    G: f32,
    softening: f32,
    dt: f32,
    particle_count: u32,
    damping: f32,
}

@group(0) @binding(0) var<storage, read_write> particles: array<Particle>;
@group(0) @binding(1) var<storage, read_write> particles_next: array<Particle>;
@group(0) @binding(2) var<uniform> params: SimParams;

// Shared memory for tile-based optimization
var<workgroup> shared_pos: array<vec4<f32>, 64>;

@compute @workgroup_size(64)
fn main(
    @builtin(global_invocation_id) global_id: vec3<u32>,
    @builtin(local_invocation_id) local_id: vec3<u32>,
    @builtin(workgroup_id) workgroup_id: vec3<u32>
) {
    let particle_idx = global_id.x;
    if (particle_idx >= params.particle_count) {
        return;
    }
    
    // Read current particle state
    let current_particle = particles[particle_idx];
    var pos = current_particle.position;
    var vel = current_particle.velocity;
    let mass_i = current_particle.mass;
    
    var force = vec3<f32>(0.0);
    
    // Tile-based force calculation
    let num_tiles = (params.particle_count + 63u) / 64u;
    
    for (var tile = 0u; tile < num_tiles; tile += 1u) {
        // Load tile into shared memory
        let load_idx = tile * 64u + local_id.x;
        if (load_idx < params.particle_count) {
            let loaded_particle = particles[load_idx];
            shared_pos[local_id.x] = vec4<f32>(loaded_particle.position, loaded_particle.mass);
        }
        workgroupBarrier();
        
        // Compute forces from this tile
        for (var j = 0u; j < 64u; j += 1u) {
            let tile_particle_idx = tile * 64u + j;
            if (tile_particle_idx >= params.particle_count) {
                break;
            }
            
            // Skip self-interaction
            if (tile_particle_idx == particle_idx) {
                continue;
            }
            
            let other_data = shared_pos[j];
            let other_pos = other_data.xyz;
            let mass_j = other_data.w;
            
            let delta = other_pos - pos;
            let dist_sq = dot(delta, delta) + params.softening * params.softening;
            
            // Optimized inverse distance calculation
            let inv_dist = rsqrt(dist_sq);
            let inv_dist_cube = inv_dist * inv_dist * inv_dist;
            
            // Gravitational force: F = G * m_i * m_j * r / r^3
            force += delta * (params.G * mass_i * mass_j * inv_dist_cube);
        }
        workgroupBarrier();
    }
    
    // Semi-implicit Euler integration with damping
    let acceleration = force / mass_i;
    vel += acceleration * params.dt;
    vel *= params.damping;
    
    // Update position
    pos += vel * params.dt;
    
    // Write to next frame buffer (double buffering)
    particles_next[particle_idx] = Particle(pos, vel, mass_i);
}
