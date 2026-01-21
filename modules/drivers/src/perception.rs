// Perception System - LIDAR, Camera, Depth Sensors
// Provides spatial awareness for obstacle detection and 3D mapping

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Point3D {
    pub x: f32,
    pub y: f32,
    pub z: f32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LidarScan {
    pub points: Vec<Point3D>,
    pub timestamp: f64,
    pub range_min: f32, // meters
    pub range_max: f32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DepthFrame {
    pub width: u32,
    pub height: u32,
    pub data: Vec<u16>, // depth in millimeters
    pub timestamp: f64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Obstacle {
    pub position: Point3D,
    pub size: Point3D, // bounding box
    pub distance: f32,
    pub confidence: f32,
}

/// LIDAR Driver for 2D/3D scanning
///
/// **Library Proxy Pattern**: Interfaces with LIDAR sensors
/// via serial/USB and provides point cloud data.
///
/// **Use Cases**:
/// - Autonomous navigation (obstacle detection)
/// - 3D mapping (SLAM)
/// - Collision avoidance
pub struct LidarDriver {
    last_scan: Option<LidarScan>,
    range_min: f32,
    range_max: f32,
}

impl LidarDriver {
    pub fn new(range_min: f32, range_max: f32) -> Self {
        Self {
            last_scan: None,
            range_min,
            range_max,
        }
    }

    /// Process raw LIDAR data
    pub fn process_scan(&mut self, _raw_data: &[u8], timestamp: f64) -> Result<LidarScan, String> {
        // Future: Parse actual LIDAR protocol (e.g., RPLIDAR, Velodyne)
        // Returns empty scan for now
        let scan = LidarScan {
            points: vec![],
            timestamp,
            range_min: self.range_min,
            range_max: self.range_max,
        };

        self.last_scan = Some(scan.clone());
        Ok(scan)
    }

    /// Get last scan
    pub fn get_last_scan(&self) -> Option<LidarScan> {
        self.last_scan.clone()
    }

    /// Detect obstacles from point cloud
    pub fn detect_obstacles(&self, _scan: &LidarScan) -> Vec<Obstacle> {
        // Future: Implement clustering algorithm (DBSCAN via linfa-clustering)
        // Returns empty list for now
        vec![]
    }
}

/// Depth Camera Driver
///
/// **Library Proxy Pattern**: Interfaces with depth cameras
/// (Intel RealSense, Kinect, etc.) and provides depth frames.
pub struct DepthCamera {
    width: u32,
    height: u32,
    last_frame: Option<DepthFrame>,
}

impl DepthCamera {
    pub fn new(width: u32, height: u32) -> Self {
        Self {
            width,
            height,
            last_frame: None,
        }
    }

    /// Process depth frame
    pub fn process_frame(&mut self, data: Vec<u16>, timestamp: f64) -> DepthFrame {
        let frame = DepthFrame {
            width: self.width,
            height: self.height,
            data,
            timestamp,
        };

        self.last_frame = Some(frame.clone());
        frame
    }

    /// Get last frame
    pub fn get_last_frame(&self) -> Option<DepthFrame> {
        self.last_frame.clone()
    }

    /// Convert depth frame to point cloud
    pub fn to_point_cloud(&self, _frame: &DepthFrame) -> Vec<Point3D> {
        // Future: Implement depth-to-3D projection using camera intrinsics
        // Returns empty point cloud for now
        vec![]
    }
}

impl Default for LidarDriver {
    fn default() -> Self {
        Self::new(0.1, 30.0) // 10cm to 30m range
    }
}

impl Default for DepthCamera {
    fn default() -> Self {
        Self::new(640, 480) // VGA resolution
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_lidar_creation() {
        let lidar = LidarDriver::new(0.1, 10.0);
        assert_eq!(lidar.range_min, 0.1);
        assert_eq!(lidar.range_max, 10.0);
    }

    #[test]
    fn test_depth_camera_creation() {
        let camera = DepthCamera::new(640, 480);
        assert_eq!(camera.width, 640);
        assert_eq!(camera.height, 480);
    }

    #[test]
    fn test_depth_frame_processing() {
        let mut camera = DepthCamera::new(2, 2);
        let data = vec![100, 200, 300, 400];
        let frame = camera.process_frame(data.clone(), 1.0);

        assert_eq!(frame.width, 2);
        assert_eq!(frame.height, 2);
        assert_eq!(frame.data, data);
    }
}
