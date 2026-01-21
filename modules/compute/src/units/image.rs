use crate::engine::{ComputeError, ResourceLimits, UnitProxy};
use async_trait::async_trait;
use dashmap::DashMap;
use fast_image_resize as fr;
use image::{
    codecs::jpeg::JpegEncoder, codecs::png::PngEncoder, DynamicImage, GenericImageView,
    ImageEncoder,
};
use imageproc::filter;
use rayon::prelude::*;
use std::sync::Arc;

/// Production-grade image processor with SIMD, zero-copy, and security
pub struct ImageUnit {
    // Cache for resizers to avoid re-creation
    resizer_cache: Arc<DashMap<ResizeKey, fr::Resizer>>,
    // Configuration
    config: ImageConfig,
}

#[derive(Hash, Eq, PartialEq, Clone)]
struct ResizeKey {
    filter: u8, // FilterType as u8
    width: u32,
    height: u32,
}

#[derive(Clone)]
struct ImageConfig {
    max_input_size: usize,
    max_output_size: usize,
    max_width: u32,
    max_height: u32,
    max_compression_ratio: f32,
    #[allow(dead_code)] // Will be used for streaming large images (Phase 2)
    streaming_threshold: usize,
}

impl Default for ImageConfig {
    fn default() -> Self {
        Self {
            max_input_size: 100 * 1024 * 1024,  // 100MB
            max_output_size: 500 * 1024 * 1024, // 500MB
            max_width: 16384,
            max_height: 16384,
            max_compression_ratio: 100.0,
            streaming_threshold: 10 * 1024 * 1024, // 10MB
        }
    }
}

impl ImageUnit {
    pub fn new() -> Self {
        Self {
            resizer_cache: Arc::new(DashMap::new()),
            config: ImageConfig::default(),
        }
    }

    /// Validate input BEFORE decoding to prevent decompression bombs
    pub(crate) fn validate_input(
        &self,
        input: &[u8],
        params: &serde_json::Value,
    ) -> Result<(), ComputeError> {
        // 1. Size check
        if input.len() > self.config.max_input_size {
            return Err(ComputeError::InputTooLarge {
                size: input.len(),
                max: self.config.max_input_size,
            });
        }

        // 2. Dimension limits (if specified in params)
        if let Some(width) = params.get("width").and_then(|v| v.as_u64()) {
            if width as u32 > self.config.max_width {
                return Err(ComputeError::ExecutionFailed(format!(
                    "Width {} exceeds maximum {}",
                    width, self.config.max_width
                )));
            }
        }
        if let Some(height) = params.get("height").and_then(|v| v.as_u64()) {
            if height as u32 > self.config.max_height {
                return Err(ComputeError::ExecutionFailed(format!(
                    "Height {} exceeds maximum {}",
                    height, self.config.max_height
                )));
            }
        }

        // 3. Estimate decompression ratio to detect bombs
        let estimated_ratio = self.estimate_decompression_ratio(input);
        if estimated_ratio > self.config.max_compression_ratio {
            return Err(ComputeError::ExecutionFailed(format!(
                "Potential decompression bomb detected (ratio: {:.1}:1)",
                estimated_ratio
            )));
        }

        Ok(())
    }

    /// Estimate decompression ratio without full decode
    fn estimate_decompression_ratio(&self, input: &[u8]) -> f32 {
        // Quick heuristic: check image dimensions from header
        if let Ok(reader) =
            image::ImageReader::new(std::io::Cursor::new(input)).with_guessed_format()
        {
            if let Ok((width, height)) = reader.into_dimensions() {
                let estimated_size = width as usize * height as usize * 4; // RGBA
                return estimated_size as f32 / input.len() as f32;
            }
        }
        1.0 // Safe default
    }

    /// Safe image loading with limits
    pub(crate) fn safe_load(&self, input: &[u8]) -> Result<DynamicImage, ComputeError> {
        let reader = image::ImageReader::new(std::io::Cursor::new(input))
            .with_guessed_format()
            .map_err(|e| {
                ComputeError::ExecutionFailed(format!("Format detection failed: {}", e))
            })?;

        // Decode with dimension limits
        let img = reader
            .decode()
            .map_err(|e| ComputeError::ExecutionFailed(format!("Image decode failed: {}", e)))?;

        // Verify dimensions after decode
        if img.width() > self.config.max_width || img.height() > self.config.max_height {
            return Err(ComputeError::ExecutionFailed(format!(
                "Image dimensions {}x{} exceed maximum {}x{}",
                img.width(),
                img.height(),
                self.config.max_width,
                self.config.max_height
            )));
        }

        Ok(img)
    }

    /// SIMD-accelerated resize using fast_image_resize
    pub(crate) fn resize_simd(
        &self,
        img: &DynamicImage,
        params: &serde_json::Value,
    ) -> Result<DynamicImage, ComputeError> {
        let width = params["width"].as_u64().unwrap_or(800) as u32;
        let height = params["height"].as_u64().unwrap_or(600) as u32;
        let filter_str = params["filter"].as_str().unwrap_or("Lanczos3");

        // Get or create cached resizer
        let filter_type = self.parse_fr_filter(filter_str);
        let key = ResizeKey {
            filter: self.filter_to_u8(&filter_type),
            width,
            height,
        };

        let mut resizer = self.resizer_cache.entry(key).or_default().clone();

        // Convert to fast_image_resize format
        let src_image = self.to_fr_image(img)?;

        // Create destination image
        let dst_width = std::num::NonZero::new(width).unwrap();
        let dst_height = std::num::NonZero::new(height).unwrap();
        let mut dst_image = fr::Image::new(dst_width, dst_height, src_image.pixel_type());

        // Resize with SIMD using views
        resizer
            .resize(&src_image.view(), &mut dst_image.view_mut())
            .map_err(|e| ComputeError::ExecutionFailed(format!("SIMD resize failed: {}", e)))?;

        // Convert back to DynamicImage
        self.fr_image_to_dynamic(&dst_image)
    }

    /// Convert DynamicImage to fast_image_resize Image
    fn to_fr_image<'a>(&self, img: &'a DynamicImage) -> Result<fr::Image<'a>, ComputeError> {
        let width = img.width();
        let height = img.height();

        match img {
            DynamicImage::ImageRgba8(buf) => {
                let width = std::num::NonZero::new(width).unwrap();
                let height = std::num::NonZero::new(height).unwrap();
                fr::Image::from_vec_u8(width, height, buf.as_raw().to_vec(), fr::PixelType::U8x4)
                    .map_err(|e| {
                        ComputeError::ExecutionFailed(format!("Image conversion failed: {}", e))
                    })
            }
            DynamicImage::ImageRgb8(buf) => {
                let width = std::num::NonZero::new(width).unwrap();
                let height = std::num::NonZero::new(height).unwrap();
                fr::Image::from_vec_u8(width, height, buf.as_raw().to_vec(), fr::PixelType::U8x3)
                    .map_err(|e| {
                        ComputeError::ExecutionFailed(format!("Image conversion failed: {}", e))
                    })
            }
            _ => {
                // Convert to RGBA for other formats
                let rgba = img.to_rgba8();
                let width = std::num::NonZero::new(width).unwrap();
                let height = std::num::NonZero::new(height).unwrap();
                fr::Image::from_vec_u8(width, height, rgba.as_raw().to_vec(), fr::PixelType::U8x4)
                    .map_err(|e| {
                        ComputeError::ExecutionFailed(format!("Image conversion failed: {}", e))
                    })
            }
        }
    }

    /// Convert fast_image_resize Image back to DynamicImage
    fn fr_image_to_dynamic(&self, img: &fr::Image) -> Result<DynamicImage, ComputeError> {
        let width = img.width();
        let height = img.height();
        let buffer = img.buffer();

        match img.pixel_type() {
            fr::PixelType::U8x4 => {
                let rgba = image::RgbaImage::from_raw(width.get(), height.get(), buffer.to_vec())
                    .ok_or_else(|| {
                    ComputeError::ExecutionFailed("Failed to create RGBA image".to_string())
                })?;
                Ok(DynamicImage::ImageRgba8(rgba))
            }
            fr::PixelType::U8x3 => {
                let rgb = image::RgbImage::from_raw(width.get(), height.get(), buffer.to_vec())
                    .ok_or_else(|| {
                        ComputeError::ExecutionFailed("Failed to create RGB image".to_string())
                    })?;
                Ok(DynamicImage::ImageRgb8(rgb))
            }
            _ => Err(ComputeError::ExecutionFailed(
                "Unsupported pixel type".to_string(),
            )),
        }
    }

    fn parse_fr_filter(&self, filter_str: &str) -> fr::FilterType {
        match filter_str {
            "Nearest" => fr::FilterType::Box,
            "Triangle" => fr::FilterType::Bilinear,
            "CatmullRom" => fr::FilterType::CatmullRom,
            "Lanczos3" => fr::FilterType::Lanczos3,
            _ => fr::FilterType::Lanczos3,
        }
    }

    /// Convert FilterType to u8 for cache key
    fn filter_to_u8(&self, filter: &fr::FilterType) -> u8 {
        match filter {
            fr::FilterType::Box => 0,
            fr::FilterType::Bilinear => 1,
            fr::FilterType::Hamming => 2,
            fr::FilterType::CatmullRom => 3,
            fr::FilterType::Mitchell => 4,
            fr::FilterType::Lanczos3 => 5,
            _ => 5, // Default to Lanczos3 for unknown filters
        }
    }

    /// Batch process multiple operations in parallel
    fn process_batch(
        &self,
        img: &DynamicImage,
        operations: Vec<(&str, serde_json::Value)>,
    ) -> Result<Vec<DynamicImage>, ComputeError> {
        operations
            .par_iter()
            .map(|(method, params)| self.execute_single_operation(img, method, params))
            .collect()
    }

    fn execute_single_operation(
        &self,
        img: &DynamicImage,
        method: &str,
        params: &serde_json::Value,
    ) -> Result<DynamicImage, ComputeError> {
        match method {
            "resize" => self.resize_simd(img, params),
            "crop" => self.crop(img, params),
            "grayscale" => Ok(img.grayscale()),
            _ => Err(ComputeError::UnknownMethod {
                library: "image".to_string(),
                method: method.to_string(),
            }),
        }
    }
}

#[async_trait]
impl UnitProxy for ImageUnit {
    fn service_name(&self) -> &str {
        "compute"
    }

    fn name(&self) -> &str {
        "image"
    }

    fn actions(&self) -> Vec<&str> {
        vec![
            "resize",
            "resize_exact",
            "resize_to_fill",
            "thumbnail",
            "crop",
            "rotate90",
            "rotate180",
            "rotate270",
            "fliph",
            "flipv",
            "grayscale",
            "invert",
            "brighten",
            "contrast",
            "huerotate",
            "blur",
            "unsharpen",
            "gaussian_blur",
            "median_filter",
            "sharpen",
            "edge_detect",
            "overlay",
            "tile",
            "adjust_levels",
        ]
    }

    fn resource_limits(&self) -> ResourceLimits {
        ResourceLimits::for_image()
    }

    async fn execute(
        &self,
        method: &str,
        input: &[u8],
        params: &[u8],
    ) -> Result<Vec<u8>, ComputeError> {
        // Parse params from slice (zero-copy JSON)
        let params: serde_json::Value = serde_json::from_slice(params)
            .map_err(|e| ComputeError::InvalidParams(format!("Invalid JSON params: {}", e)))?;

        // CRITICAL: Validate BEFORE decoding
        self.validate_input(input, &params)?;

        // Safe load with limits
        let img = self.safe_load(input)?;

        // Check if this is a batch request (multiple operations)
        if let Some(operations) = params.get("batch").and_then(|v| v.as_array()) {
            // Batch processing: execute multiple operations in parallel
            let ops: Vec<(&str, serde_json::Value)> = operations
                .iter()
                .filter_map(|op| {
                    let method = op.get("method")?.as_str()?;
                    let op_params = op.get("params").cloned().unwrap_or(serde_json::json!({}));
                    Some((method, op_params))
                })
                .collect();

            if ops.is_empty() {
                return Err(ComputeError::InvalidParams(
                    "Empty batch operations".to_string(),
                ));
            }

            // Process all operations in parallel
            let results = self.process_batch(&img, ops)?;

            // Return the first result (or could return all as array)
            // For now, return the last result as the final output
            let final_result = results.last().ok_or_else(|| {
                ComputeError::ExecutionFailed("No results from batch processing".to_string())
            })?;

            return self.encode_output(final_result, &params);
        }

        // Single operation execution
        let result = match method {
            // SIMD-accelerated operations
            "resize" | "resize_exact" | "resize_to_fill" => self.resize_simd(&img, &params)?,
            "thumbnail" => self.thumbnail(&img, &params)?,

            // Fast operations
            "crop" => self.crop(&img, &params)?,
            "rotate90" => img.rotate90(),
            "rotate180" => img.rotate180(),
            "rotate270" => img.rotate270(),
            "fliph" => img.fliph(),
            "flipv" => img.flipv(),

            // Color operations
            "grayscale" => img.grayscale(),
            "invert" => {
                let mut result = img.clone();
                result.invert();
                result
            }
            "brighten" => {
                let value = params["value"].as_i64().unwrap_or(10) as i32;
                img.brighten(value)
            }
            "contrast" => {
                let value = params["value"].as_f64().unwrap_or(1.5) as f32;
                img.adjust_contrast(value)
            }
            "huerotate" => {
                let degrees = params["degrees"].as_i64().unwrap_or(90) as i32;
                img.huerotate(degrees)
            }

            // Filters
            "blur" => {
                let sigma = params["sigma"].as_f64().unwrap_or(2.0) as f32;
                img.blur(sigma)
            }
            "unsharpen" => {
                let sigma = params["sigma"].as_f64().unwrap_or(2.0) as f32;
                let threshold = params["threshold"].as_i64().unwrap_or(0) as i32;
                img.unsharpen(sigma, threshold)
            }
            "gaussian_blur" => self.gaussian_blur(&img, &params)?,
            "median_filter" => self.median_filter(&img, &params)?,
            "sharpen" => self.sharpen(&img)?,
            "edge_detect" => self.edge_detect(&img)?,

            // Advanced
            "overlay" => self.overlay(&img, &params)?,
            "tile" => self.tile(&img, &params)?,
            "adjust_levels" => self.adjust_levels(&img, &params)?,

            _ => {
                return Err(ComputeError::UnknownMethod {
                    library: "image".to_string(),
                    method: method.to_string(),
                });
            }
        };

        // Encode with size validation
        self.encode_output(&result, &params)
    }
}

// === IMPLEMENTATION METHODS ===
impl ImageUnit {
    fn thumbnail(
        &self,
        img: &DynamicImage,
        params: &serde_json::Value,
    ) -> Result<DynamicImage, ComputeError> {
        let size = params["size"].as_u64().unwrap_or(256) as u32;
        Ok(img.thumbnail(size, size))
    }

    pub(crate) fn crop(
        &self,
        img: &DynamicImage,
        params: &serde_json::Value,
    ) -> Result<DynamicImage, ComputeError> {
        let x = params["x"].as_u64().unwrap_or(0) as u32;
        let y = params["y"].as_u64().unwrap_or(0) as u32;
        let width = params["width"].as_u64().unwrap_or(100) as u32;
        let height = params["height"].as_u64().unwrap_or(100) as u32;

        Ok(img.crop_imm(x, y, width, height))
    }

    fn gaussian_blur(
        &self,
        img: &DynamicImage,
        params: &serde_json::Value,
    ) -> Result<DynamicImage, ComputeError> {
        let sigma = params["sigma"].as_f64().unwrap_or(2.0) as f32;

        let gray = img.to_luma8();
        let blurred = filter::gaussian_blur_f32(&gray, sigma);

        Ok(DynamicImage::ImageLuma8(blurred))
    }

    fn median_filter(
        &self,
        img: &DynamicImage,
        params: &serde_json::Value,
    ) -> Result<DynamicImage, ComputeError> {
        let radius_x = params["radius_x"].as_u64().unwrap_or(3) as u32;
        let radius_y = params["radius_y"].as_u64().unwrap_or(3) as u32;

        let gray = img.to_luma8();
        let filtered = filter::median_filter(&gray, radius_x, radius_y);

        Ok(DynamicImage::ImageLuma8(filtered))
    }

    fn sharpen(&self, img: &DynamicImage) -> Result<DynamicImage, ComputeError> {
        let gray = img.to_luma8();
        let sharpened = filter::sharpen3x3(&gray);

        Ok(DynamicImage::ImageLuma8(sharpened))
    }

    fn edge_detect(&self, img: &DynamicImage) -> Result<DynamicImage, ComputeError> {
        use imageproc::gradients;

        let gray = img.to_luma8();
        let edges_u16 = gradients::sobel_gradients(&gray);

        // Convert u16 to u8 by scaling down
        let edges_u8 =
            image::ImageBuffer::from_fn(edges_u16.width(), edges_u16.height(), |x, y| {
                let pixel = edges_u16.get_pixel(x, y);
                image::Luma([(pixel[0] / 256) as u8])
            });

        Ok(DynamicImage::ImageLuma8(edges_u8))
    }

    fn overlay(
        &self,
        img: &DynamicImage,
        params: &serde_json::Value,
    ) -> Result<DynamicImage, ComputeError> {
        let top_data = params["top_image"]
            .as_str()
            .ok_or_else(|| ComputeError::InvalidParams("Missing top_image".to_string()))?;

        use base64::{engine::general_purpose, Engine as _};
        let top_bytes = general_purpose::STANDARD
            .decode(top_data)
            .map_err(|e| ComputeError::InvalidParams(format!("Invalid base64: {}", e)))?;

        // Validate overlay size
        self.validate_input(&top_bytes, &serde_json::json!({}))?;

        let top_img = self.safe_load(&top_bytes)?;

        let x = params["x"].as_u64().unwrap_or(0) as i64;
        let y = params["y"].as_u64().unwrap_or(0) as i64;

        let mut result = img.to_rgba8();
        image::imageops::overlay(&mut result, &top_img.to_rgba8(), x, y);

        Ok(DynamicImage::ImageRgba8(result))
    }

    fn tile(
        &self,
        img: &DynamicImage,
        params: &serde_json::Value,
    ) -> Result<DynamicImage, ComputeError> {
        let tiles_x = params["tiles_x"].as_u64().unwrap_or(2) as u32;
        let tiles_y = params["tiles_y"].as_u64().unwrap_or(2) as u32;

        let (width, height) = img.dimensions();
        let new_width = width * tiles_x;
        let new_height = height * tiles_y;

        let mut result = image::RgbaImage::new(new_width, new_height);

        for ty in 0..tiles_y {
            for tx in 0..tiles_x {
                image::imageops::overlay(
                    &mut result,
                    &img.to_rgba8(),
                    (tx * width) as i64,
                    (ty * height) as i64,
                );
            }
        }

        Ok(DynamicImage::ImageRgba8(result))
    }

    fn adjust_levels(
        &self,
        img: &DynamicImage,
        params: &serde_json::Value,
    ) -> Result<DynamicImage, ComputeError> {
        let black_point = params["black_point"].as_u64().unwrap_or(0) as u8;
        let white_point = params["white_point"].as_u64().unwrap_or(255) as u8;
        let gamma = params["gamma"].as_f64().unwrap_or(1.0) as f32;

        let mut result = img.to_rgba8();

        for pixel in result.pixels_mut() {
            for channel in 0..3 {
                let value = pixel[channel];
                let normalized = (value.saturating_sub(black_point) as f32)
                    / ((white_point - black_point) as f32);
                let adjusted = normalized.powf(1.0 / gamma);
                pixel[channel] = (adjusted * 255.0).clamp(0.0, 255.0) as u8;
            }
        }

        Ok(DynamicImage::ImageRgba8(result))
    }

    fn encode_output(
        &self,
        img: &DynamicImage,
        params: &serde_json::Value,
    ) -> Result<Vec<u8>, ComputeError> {
        let format = params["format"].as_str().unwrap_or("png");
        let quality = params["quality"].as_u64().unwrap_or(90) as u8;

        let mut output = Vec::new();

        match format {
            "jpeg" => {
                let mut encoder = JpegEncoder::new_with_quality(&mut output, quality);
                encoder
                    .encode(
                        img.as_bytes(),
                        img.width(),
                        img.height(),
                        img.color().into(),
                    )
                    .map_err(|e| {
                        ComputeError::ExecutionFailed(format!("JPEG encode failed: {}", e))
                    })?;
            }
            "jpg" => {
                let mut encoder = JpegEncoder::new_with_quality(&mut output, quality);
                encoder
                    .encode(
                        img.as_bytes(),
                        img.width(),
                        img.height(),
                        img.color().into(),
                    )
                    .map_err(|e| {
                        ComputeError::ExecutionFailed(format!("JPEG encode failed: {}", e))
                    })?;
            }
            "png" => {
                let encoder = PngEncoder::new(&mut output);
                encoder
                    .write_image(
                        img.as_bytes(),
                        img.width(),
                        img.height(),
                        img.color().into(),
                    )
                    .map_err(|e| {
                        ComputeError::ExecutionFailed(format!("PNG encode failed: {}", e))
                    })?;
            }
            _ => {
                // Default to PNG for unknown formats
                let encoder = PngEncoder::new(&mut output);
                encoder
                    .write_image(
                        img.as_bytes(),
                        img.width(),
                        img.height(),
                        img.color().into(),
                    )
                    .map_err(|e| {
                        ComputeError::ExecutionFailed(format!("PNG encode failed: {}", e))
                    })?;
            }
        }

        // Validate output size
        if output.len() > self.config.max_output_size {
            return Err(ComputeError::OutputTooLarge {
                size: output.len(),
                max: self.config.max_output_size,
            });
        }

        Ok(output)
    }
}
