#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const os = require('os');

const binariesDir = path.join(__dirname, 'binaries');
const platform = os.platform();
const arch = os.arch();

// Map Node.js platform/arch to Go binary naming
const platformMap = {
  'darwin': 'darwin',
  'linux': 'linux',
  'win32': 'windows'
};

const archMap = {
  'x64': 'amd64',
  'arm64': 'arm64'
};

const goPlatform = platformMap[platform];
const goArch = archMap[arch];

if (!goPlatform || !goArch) {
  console.error(`Error: Unsupported platform ${platform}/${arch}`);
  process.exit(1);
}

// Determine binary name
const isWindows = platform === 'win32';
const binaryName = isWindows ? 'port.exe' : 'port';
const sourceBinary = isWindows 
  ? `port-${goPlatform}-${goArch}.exe`
  : `port-${goPlatform}-${goArch}`;
const sourcePath = path.join(binariesDir, sourceBinary);
const targetPath = path.join(binariesDir, binaryName);

// Check if source binary exists
if (!fs.existsSync(sourcePath)) {
  console.error(`Error: Binary not found for platform ${goPlatform}/${goArch}`);
  console.error(`Expected: ${sourcePath}`);
  process.exit(1);
}

// Copy the correct binary to the target location
try {
  fs.copyFileSync(sourcePath, targetPath);
  
  // Set executable permissions on Unix systems
  if (!isWindows) {
    fs.chmodSync(targetPath, 0o755);
  }
  
  // Delete other platform binaries to reduce package size
  const files = fs.readdirSync(binariesDir);
  files.forEach(file => {
    const filePath = path.join(binariesDir, file);
    const stat = fs.statSync(filePath);
    
    if (stat.isFile() && file !== binaryName && file.startsWith('port-')) {
      fs.unlinkSync(filePath);
    }
  });
  
  console.log(`✓ Installed Port CLI for ${goPlatform}/${goArch}`);
} catch (error) {
  console.error(`Error installing binary: ${error.message}`);
  process.exit(1);
}
