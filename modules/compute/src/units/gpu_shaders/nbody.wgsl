// Enhanced N-Body Gravitational Simulation
// Features: Multiple force laws, particle types, collisions, rendering effects

struct Particle {
    position: vec3<f32>,
    velocity: vec3<f32>,
    acceleration: vec3<f32>,
    mass: f32,
    radius: f32,
    color: vec4<f32>,
    temperature: f32,
    luminosity: f32,
    particle_type: u32,  // 0=normal, 1=star, 2=blackhole, 3=dark matter
    lifetime: f32,
    angular_velocity: vec3<f32>,
}

struct SimParams {
    // Core physics
    G: f32,
    dt: f32,
    particle_count: u32,
    
    // Force options
    softening: f32,
    force_law: u32,      // 0=Newtonian, 1=Plummer, 2=Cubic, 3=Logarithmic
    dark_matter_factor: f32,
    cosmic_expansion: f32,
    
    // Interaction options
    enable_collisions: u32,
    merge_threshold: f32,
    restitution: f32,
    tidal_forces: u32,
    
    // Effects
    drag_coefficient: f32,
    turbulence_strength: f32,
    turbulence_scale: f32,
    magnetic_strength: f32,
    radiation_pressure: f32,
    
    // Cosmology
    universe_radius: f32,
    background_density: f32,
    
    // Random seed
    time: f32,
    seed: vec4<f32>,
}

struct GalaxyArms {
    arm_count: u32,
    arm_tightness: f32,
    arm_width: f32,
    rotation_speed: f32,
}

// Particle buffers (double buffering)
@group(0) @binding(0) var<storage, read_write> particles_a: array<Particle>;
@group(0) @binding(1) var<storage, read_write> particles_b: array<Particle>;
@group(0) @binding(2) var<uniform> params: SimParams;
@group(0) @binding(3) var<uniform> galaxy: GalaxyArms;

// Shared memory for optimized force calculation
var<workgroup> shared_pos_mass: array<vec4<f32>, 256>;
var<workgroup> shared_type_temp: array<vec4<f32>, 256>;

// Random number generator
fn rand(uv: vec2<f32>, seed: f32) -> f32 {
    return fract(sin(dot(uv, vec2<f32>(12.9898, 78.233)) + seed) * 43758.5453);
}

// 3D noise for turbulence
fn turbulence(p: vec3<f32>, scale: f32) -> vec3<f32> {
    let n1 = sin(dot(p, vec3<f32>(127.1, 311.7, 74.7))) * 43758.5453;
    let n2 = sin(dot(p, vec3<f32>(269.5, 183.3, 246.1))) * 43758.5453;
    let n3 = sin(dot(p, vec3<f32>(113.5, 271.9, 124.6))) * 43758.5453;
    return fract(vec3<f32>(n1, n2, n3)) * 2.0 - 1.0;
}

// Color based on temperature (blackbody radiation)
fn temperature_to_color(temp: f32) -> vec4<f32> {
    var t = temp * 0.001;
    t = clamp(t, 0.1, 1.0);
    
    var color = vec3<f32>(0.0);
    
    if (t < 0.4) {
        let x = t / 0.4;
        color = mix(vec3<f32>(0.0, 0.2, 1.0), 
                    vec3<f32>(0.8, 0.8, 1.0), x);
    } else if (t < 0.7) {
        let x = (t - 0.4) / 0.3;
        color = mix(vec3<f32>(0.8, 0.8, 1.0),
                    vec3<f32>(1.0, 1.0, 0.5), x);
    } else {
        let x = (t - 0.7) / 0.3;
        color = mix(vec3<f32>(1.0, 1.0, 0.5),
                    vec3<f32>(1.0, 0.3, 0.1), x);
    }
    
    return vec4<f32>(color, 1.0);
}

// Different force laws
fn calculate_force(dist_sq: f32, delta: vec3<f32>, mass_i: f32, mass_j: f32, law: u32) -> vec3<f32> {
    var force = vec3<f32>(0.0);
    
    switch(law) {
        case 0u: { // Newtonian
            let inv_dist = inverseSqrt(dist_sq + params.softening);
            let inv_dist_cube = inv_dist * inv_dist * inv_dist;
            force = delta * (params.G * mass_i * mass_j * inv_dist_cube);
        }
        case 1u: { // Plummer sphere
            let r = sqrt(dist_sq);
            let r_core = params.softening;
            let denom = pow(r * r + r_core * r_core, 1.5);
            force = delta * (params.G * mass_i * mass_j / denom);
        }
        case 2u: { // Cubic falloff
            let r = sqrt(dist_sq);
            let r_core = params.softening;
            let denom = (r + r_core) * (r + r_core) * (r + r_core);
            force = delta * (params.G * mass_i * mass_j / denom);
        }
        case 3u: { // Logarithmic
            let r = sqrt(dist_sq + params.softening);
            force = delta * (params.G * mass_i * mass_j / (r * r)) * 
                    (1.0 - exp(-r / params.softening));
        }
        default: {
            let inv_dist = inverseSqrt(dist_sq + params.softening);
            let inv_dist_cube = inv_dist * inv_dist * inv_dist;
            force = delta * (params.G * mass_i * mass_j * inv_dist_cube);
        }
    }
    
    return force;
}

// Galaxy arm density function
fn galaxy_arm_density(pos: vec3<f32>) -> f32 {
    let r = length(pos.xz);
    let theta = atan2(pos.z, pos.x);
    
    var density = 0.0;
    for (var i = 0u; i < galaxy.arm_count; i += 1u) {
        let arm_angle = f32(i) * (2.0 * 3.14159 / f32(galaxy.arm_count));
        let arm_theta = theta + galaxy.rotation_speed * params.time;
        let phase_diff = sin(arm_theta - arm_angle + galaxy.arm_tightness * log(r + 1.0));
        density += exp(-phase_diff * phase_diff / (galaxy.arm_width * galaxy.arm_width));
    }
    
    density *= exp(-r * r / (params.universe_radius * params.universe_radius * 0.25));
    
    return density;
}

@compute @workgroup_size(256)
fn main(
    @builtin(global_invocation_id) global_id: vec3<u32>,
    @builtin(local_invocation_id) local_id: vec3<u32>,
    @builtin(workgroup_id) workgroup_id: vec3<u32>
) {
    let particle_idx = global_id.x;
    if (particle_idx >= params.particle_count) {
        return;
    }
    
    var particle = particles_a[particle_idx];
    var pos = particle.position;
    var vel = particle.velocity;
    let mass_i = particle.mass;
    let type_i = particle.particle_type;
    let radius_i = particle.radius;
    
    var force = vec3<f32>(0.0);
    var has_collided = false;
    var collision_impulse = vec3<f32>(0.0);
    
    // Tile-based force calculation
    let num_tiles = (params.particle_count + 255u) / 256u;
    
    for (var tile = 0u; tile < num_tiles; tile += 1u) {
        let load_idx = tile * 256u + local_id.x;
        if (load_idx < params.particle_count) {
            let loaded = particles_a[load_idx];
            shared_pos_mass[local_id.x] = vec4<f32>(loaded.position, loaded.mass);
            shared_type_temp[local_id.x] = vec4<f32>(
                f32(loaded.particle_type),
                loaded.temperature,
                loaded.radius,
                loaded.luminosity
            );
        }
        workgroupBarrier();
        
        for (var j = 0u; j < 256u; j += 1u) {
            let tile_idx = tile * 256u + j;
            if (tile_idx >= params.particle_count || tile_idx == particle_idx) {
                continue;
            }
            
            let other_pos_mass = shared_pos_mass[j];
            let other_data = shared_type_temp[j];
            let other_pos = other_pos_mass.xyz;
            let mass_j = other_pos_mass.w;
            let type_j = u32(other_data.x);
            let radius_j = other_data.z;
            
            let delta = other_pos - pos;
            let dist_sq = dot(delta, delta);
            let dist = sqrt(dist_sq);
            
            // Collision detection
            if (params.enable_collisions == 1u && dist < (radius_i + radius_j) * params.merge_threshold) {
                has_collided = true;
                
                if (dist > 0.001) {
                    let normal = normalize(delta);
                    let other_vel = particles_a[tile_idx].velocity;
                    let rel_vel = vel - other_vel;
                    let vel_normal = dot(rel_vel, normal);
                    
                    if (vel_normal < 0.0) {
                        let impulse_mag = -(1.0 + params.restitution) * vel_normal;
                        collision_impulse += impulse_mag * normal * mass_j;
                        
                        if (mass_i < 10.0 && mass_j < 10.0 && dist < (radius_i + radius_j) * 0.5) {
                            particle.mass += mass_j * 0.5;
                            particle.radius = pow(particle.mass, 0.333);
                            particle.temperature = mix(particle.temperature, other_data.y, 0.5);
                        }
                    }
                }
            }
            
            // Gravitational force
            if (type_i == 2u && type_j == 2u) {
                continue;
            }
            
            var force_factor = 1.0;
            
            if (type_i == 3u || type_j == 3u) {
                force_factor = params.dark_matter_factor;
                if (type_i == 3u && type_j == 3u) {
                    continue;
                }
            }
            
            if (type_j == 2u) {
                force_factor *= 10.0;
                let angular_momentum = cross(pos, vel);
                let accretion_force = cross(vel, angular_momentum) * 0.01;
                force += accretion_force;
            }
            
            if (dist_sq > 0.001) {
                force += calculate_force(dist_sq, delta, mass_i, mass_j, params.force_law) * force_factor;
                
                if (params.tidal_forces == 1u && dist < 10.0 * (radius_i + radius_j)) {
                    let tidal_force = delta * (mass_j / (dist * dist * dist)) * 0.1;
                    force += tidal_force;
                }
            }
        }
        workgroupBarrier();
    }
    
    if (has_collided) {
        vel += collision_impulse / mass_i;
    }
    
    // Additional effects
    let arm_density = galaxy_arm_density(pos);
    let arm_force = -normalize(pos) * arm_density * 0.1;
    force += arm_force;
    
    if (params.turbulence_strength > 0.0) {
        let turb = turbulence(pos * params.turbulence_scale, params.turbulence_scale);
        force += turb * params.turbulence_strength * mass_i;
    }
    
    if (params.magnetic_strength > 0.0) {
        let B = vec3<f32>(
            sin(pos.y * 0.1 + params.time),
            sin(pos.z * 0.1 + params.time),
            sin(pos.x * 0.1 + params.time)
        ) * params.magnetic_strength;
        let lorentz_force = cross(vel, B) * 0.01;
        force += lorentz_force;
    }
    
    if (params.radiation_pressure > 0.0 && particle.luminosity > 0.0) {
        let rad_force = normalize(pos) * particle.luminosity * params.radiation_pressure;
        force += rad_force;
    }
    
    let drag_force = -vel * length(vel) * params.drag_coefficient;
    force += drag_force;
    
    if (params.cosmic_expansion > 0.0) {
        let expansion_vel = pos * params.cosmic_expansion;
        vel += expansion_vel * params.dt;
    }
    
    // Integration
    let accel = force / max(mass_i, 0.001);
    particle.acceleration = accel;
    
    vel += accel * params.dt;
    pos += vel * params.dt;
    
    // Boundary conditions
    if (length(pos) > params.universe_radius) {
        pos = -pos * 0.9;
        vel *= -0.5;
    }
    
    // Update properties
    let speed_sq = dot(vel, vel);
    particle.temperature = mix(particle.temperature, speed_sq * 0.01, 0.1);
    
    particle.luminosity = particle.temperature * particle.temperature * particle.temperature *
                         particle.temperature * particle.mass * 0.000001;
    
    // Update color
    if (type_i == 2u) {
        particle.color = vec4<f32>(0.05, 0.05, 0.1, 1.0);
    } else if (type_i == 3u) {
        particle.color = vec4<f32>(0.3, 0.1, 0.5, 0.3);
    } else {
        particle.color = temperature_to_color(particle.temperature);
        let radial_vel = dot(normalize(pos), vel);
        let doppler = 1.0 + radial_vel * 0.01;
        particle.color = vec4<f32>(
            particle.color.r * clamp(doppler, 0.8, 1.2),
            particle.color.g,
            particle.color.b * clamp(1.0 / doppler, 0.8, 1.2),
            particle.color.a
        );
    }
    
    if (length(pos) > 0.1) {
        let angular_momentum = cross(pos, vel);
        particle.angular_velocity = angular_momentum / (length(pos) * length(pos));
    }
    
    particle.lifetime -= params.dt;
    if (particle.lifetime <= 0.0 && particle.particle_type != 2u) {
        let angle = rand(vec2<f32>(f32(particle_idx), params.time), params.seed.x);
        let radius = rand(vec2<f32>(f32(particle_idx), params.time + 1.0), params.seed.y) * 
                     params.universe_radius * 0.5;
        
        pos = vec3<f32>(
            radius * cos(angle),
            radius * sin(angle) * 0.1,
            radius * sin(angle) * 0.3
        );
        
        let orbital_speed = sqrt(params.G * 1000.0 / radius);
        vel = vec3<f32>(-pos.z, 0.0, pos.x) * orbital_speed / length(pos);
        
        particle.temperature = 5000.0 + rand(vec2<f32>(f32(particle_idx), params.time + 2.0), params.seed.z) * 10000.0;
        particle.lifetime = 100.0 + rand(vec2<f32>(f32(particle_idx), params.time + 3.0), params.seed.w) * 900.0;
    }
    
    particle.position = pos;
    particle.velocity = vel;
    
    particles_b[particle_idx] = particle;
}

@compute @workgroup_size(256)
fn compute_rendering_effects(
    @builtin(global_invocation_id) global_id: vec3<u32>
) {
    let particle_idx = global_id.x;
    if (particle_idx >= params.particle_count) {
        return;
    }
    
    var particle = particles_b[particle_idx];
    
    // Bloom effect
    if (particle.luminosity > 0.5) {
        let bloom_factor = sqrt(particle.luminosity);
        particle.radius *= (1.0 + bloom_factor * 0.5);
        particle.color.a = min(particle.color.a * (1.0 + bloom_factor), 1.0);
    }
    
    // Age-based fading
    let age_factor = particle.lifetime / 100.0;
    particle.color.a *= age_factor;
    
    // Type-specific effects
    switch(particle.particle_type) {
        case 1u: { // Stars: twinkle
            let twinkle = sin(params.time * 5.0 + f32(particle_idx)) * 0.1 + 0.9;
            particle.color = vec4<f32>(particle.color.rgb * twinkle, particle.color.a);
        }
        case 2u: { // Black holes: accretion disk glow
            let glow = 0.1 + 0.05 * sin(params.time * 2.0 + f32(particle_idx));
            particle.color = vec4<f32>(
                particle.color.rgb + vec3<f32>(glow, glow * 0.5, glow) * 0.3,
                particle.color.a
            );
        }
        default: {}
    }
    
    particles_b[particle_idx] = particle;
}
