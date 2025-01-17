#!/usr/bin/env node
const fs = require('fs');
const path = require('path');
const os = require('os');

const GITHUB_REPO = 'GNURub/node-package-updater';
const CLI_NAME = 'npu';

// Mapeo de plataformas
const platforms = {
  darwin: 'darwin',
  linux: 'linux',
  win32: 'windows',
};

const arches = {
  x64: 'amd64',
  arm64: 'arm64',
};

async function getLatestVersion() {
  const response = await fetch(
    `https://api.github.com/repos/${GITHUB_REPO}/releases/latest`,
    {
      headers: { 'User-Agent': 'Node.js' },
    }
  );

  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`);
  }

  const data = await response.json();
  return data.tag_name;
}

async function downloadBinary(url, dest) {
  const response = await fetch(url, { redirect: 'follow' });

  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`);
  }

  const buffer = await response.arrayBuffer();
  fs.writeFileSync(dest, Buffer.from(buffer));
}

async function install() {
  try {
    const platform = platforms[os.platform()];
    const arch = arches[os.arch()];

    if (!platform || !arch) {
      throw new Error(`Not supported platform: ${os.platform()} ${os.arch()}`);
    }

    const binPath = path.join(__dirname);
    if (!fs.existsSync(binPath)) {
      fs.mkdirSync(binPath, { recursive: true });
    }

    const version = await getLatestVersion();

    const binaryName = `${CLI_NAME}_${platform}_${arch}${
      platform === 'windows' ? '.exe' : ''
    }`;
    const downloadUrl = `https://github.com/${GITHUB_REPO}/releases/download/${version}/${binaryName}`;

    const binaryPath = path.join(binPath, binaryName);

    console.log(`📦 Downloading ${binaryName}...`);
    await downloadBinary(downloadUrl, binaryPath);

    if (platform !== 'windows') {
      fs.chmodSync(binaryPath, '755');
    }

    // Crear el script wrapper
    const wrapperPath = path.join(binPath, 'run.js');

    const wrapperContent = `#!/usr/bin/env node
const { spawn } = require('child_process');
const path = require('path');

const binaryPath = path.join(__dirname, '${binaryName}');
const child = spawn(binaryPath, process.argv.slice(2), { stdio: 'inherit' });

child.on('exit', (code) => process.exit(code));`;

    fs.writeFileSync(wrapperPath, wrapperContent);
    fs.chmodSync(wrapperPath, '755');

    console.log('✅ Succeded!');
  } catch (error) {
    console.error('❌ Installation error:', error.message);
    process.exit(1);
  }
}

install();
