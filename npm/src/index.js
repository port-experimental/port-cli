#!/usr/bin/env node

const { spawn } = require('child_process');
const path = require('path');
const os = require('os');

const platform = os.platform();
const isWindows = platform === 'win32';
const binaryName = isWindows ? 'port.exe' : 'port';
const binaryPath = path.join(__dirname, '..', 'binaries', binaryName);

// Get all command-line arguments (skip node and script path)
const args = process.argv.slice(2);

// Spawn the Go binary with all arguments
const child = spawn(binaryPath, args, {
  stdio: 'inherit',
  shell: false
});

// Forward exit code
child.on('exit', (code) => {
  process.exit(code !== null ? code : 1);
});

// Handle errors
child.on('error', (error) => {
  console.error(`Error executing Port CLI: ${error.message}`);
  process.exit(1);
});
