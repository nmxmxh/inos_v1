#!/usr/bin/env node
/**
 * Cap'n Proto Constants TypeScript Generator
 * 
 * Generic script to extract const values from ANY Cap'n Proto schema
 * and generate TypeScript exports alongside capnp-es generated code.
 * 
 * Output: frontend/bridge/generated/<schema_path>.consts.ts
 * 
 * Usage: 
 *   node scripts/gen-capnp-consts-ts.js <schema.capnp>
 *   node scripts/gen-capnp-consts-ts.js protocols/schemas/system/v1/sab_layout.capnp
 */

const fs = require('fs');
const path = require('path');

const BRIDGE_GENERATED_DIR = 'frontend/bridge/generated';

/**
 * Parse Cap'n Proto const declarations
 * Format: const name :Type = value; # comment
 */
function parseCapnpConsts(content) {
  const lines = content.split('\n');
  const consts = [];
  
  for (const line of lines) {
    const match = line.match(/^const\s+(\w+)\s*:\s*(\w+)\s*=\s*([^;]+);(.*)$/);
    if (match) {
      const [, name, type, rawValue, rest] = match;
      let value = rawValue.trim();
      
      // Parse value
      if (value.startsWith('0x') || value.startsWith('0X')) {
        value = parseInt(value, 16);
      } else if (value.includes('.')) {
        value = parseFloat(value);
      } else if (!isNaN(parseInt(value, 10))) {
        value = parseInt(value, 10);
      } else if (value.startsWith('"') && value.endsWith('"')) {
        value = value.slice(1, -1);
      }
      
      const commentMatch = rest.match(/#\s*(.+)/);
      const doc = commentMatch ? commentMatch[1].trim() : '';
      
      consts.push({ name, type, value, doc });
    }
  }
  
  return consts;
}

/**
 * Extract file ID from schema
 */
function extractFileId(content) {
  const match = content.match(/@(0x[a-fA-F0-9]+);/);
  return match ? match[1] : null;
}

/**
 * Convert camelCase to SCREAMING_SNAKE_CASE
 */
function toScreamingSnake(name) {
  return name
    .replace(/([a-z])([A-Z])/g, '$1_$2')
    .replace(/([A-Z]+)([A-Z][a-z])/g, '$1_$2')
    .toUpperCase();
}

/**
 * Format value for TypeScript output
 */
function formatValue(value) {
  if (typeof value === 'number' && value >= 0x1000 && Number.isInteger(value)) {
    return `0x${value.toString(16).toUpperCase().padStart(6, '0')}`;
  }
  if (typeof value === 'string') {
    return `"${value}"`;
  }
  return String(value);
}

/**
 * Generate TypeScript code - clean output without duplicates
 */
function generateTypeScript(consts, schemaPath, fileId) {
  let code = `/**
 * Cap'n Proto Constants - Auto-Generated
 * 
 * Source: ${schemaPath}
 * File ID: ${fileId || 'unknown'}
 * 
 * DO NOT EDIT MANUALLY - Regenerate with: make proto
 * 
 * @generated
 */

`;

  // Export all constants
  for (const c of consts) {
    const constName = toScreamingSnake(c.name);
    code += `/** ${c.doc || c.name} */\n`;
    code += `export const ${constName} = ${formatValue(c.value)} as const;\n\n`;
  }

  // Add a unified CONSTS object for iteration
  code += `// ========== UNIFIED CONSTANTS OBJECT ==========\n\n`;
  code += `export const CONSTS = {\n`;
  for (const c of consts) {
    code += `  ${toScreamingSnake(c.name)},\n`;
  }
  code += `} as const;\n\n`;
  code += `export type ConstKeys = keyof typeof CONSTS;\n`;

  return code;
}

/**
 * Compute output path: same structure as capnp-es output
 * Input:  protocols/schemas/system/v1/sab_layout.capnp
 * Output: frontend/bridge/generated/protocols/schemas/system/v1/sab_layout.consts.ts
 */
function getOutputPath(schemaPath) {
  const relPath = schemaPath.replace('.capnp', '.consts.ts');
  return path.join(BRIDGE_GENERATED_DIR, relPath);
}

// Main execution
function main() {
  const args = process.argv.slice(2);
  
  if (args.length === 0) {
    console.log('Usage: node scripts/gen-capnp-consts-ts.js <schema.capnp>');
    process.exit(1);
  }
  
  const schemaPath = args[0];
  const outputPath = getOutputPath(schemaPath);
  
  console.log(`🔧 Extracting constants from ${schemaPath}...`);
  
  const fullSchemaPath = path.join(process.cwd(), schemaPath);
  
  if (!fs.existsSync(fullSchemaPath)) {
    console.error(`❌ Schema not found: ${fullSchemaPath}`);
    process.exit(1);
  }
  
  const content = fs.readFileSync(fullSchemaPath, 'utf8');
  const fileId = extractFileId(content);
  const consts = parseCapnpConsts(content);
  
  if (consts.length === 0) {
    console.log(`ℹ️  No const declarations found in ${schemaPath}`);
    process.exit(0);
  }
  
  console.log(`  Found ${consts.length} constants`);
  
  const typescript = generateTypeScript(consts, schemaPath, fileId);
  
  // Ensure output directory exists
  const fullOutputPath = path.join(process.cwd(), outputPath);
  fs.mkdirSync(path.dirname(fullOutputPath), { recursive: true });
  fs.writeFileSync(fullOutputPath, typescript);
  
  console.log(`✅ Generated: ${outputPath}`);
}

main();
