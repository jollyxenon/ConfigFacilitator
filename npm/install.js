#!/usr/bin/env node

const crypto = require("node:crypto");
const fs = require("node:fs");
const https = require("node:https");
const os = require("node:os");
const path = require("node:path");
const { execFileSync } = require("node:child_process");

const packageJson = require("./package.json");

const OWNER = "jollyxenon";
const REPO = "ConfigFacilitator";
const BINARY_BASE = "cfgfc";
const PACKAGE_ROOT = __dirname;
const BIN_DIR = path.join(PACKAGE_ROOT, "bin");
const VERSION = packageJson.version;
const TAG = `v${VERSION}`;

// Tests may override platform details to exercise error paths without changing the host.
const CURRENT_PLATFORM = process.env.CFGFC_TEST_PLATFORM || process.platform;
const CURRENT_ARCH = process.env.CFGFC_TEST_ARCH || process.arch;

main().catch((error) => {
  console.error(`cfgfc install failed: ${error.message}`);
  process.exit(1);
});

async function main() {
  const target = resolveTarget(CURRENT_PLATFORM, CURRENT_ARCH);
  const binaryName = target.platform === "windows" ? `${BINARY_BASE}.exe` : BINARY_BASE;

  if (process.env.CFGFC_BINARY_PATH) {
    installLocalBinary(process.env.CFGFC_BINARY_PATH, binaryName, target.platform);
    return;
  }

  const archiveName = archiveFileName(target);
  const releaseBase = `https://github.com/${OWNER}/${REPO}/releases/download/${TAG}`;
  const archiveUrl = `${releaseBase}/${archiveName}`;
  const checksumsUrl = `${releaseBase}/checksums.txt`;

  fs.mkdirSync(BIN_DIR, { recursive: true });

  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "cfgfc-npm-"));
  const archivePath = path.join(tmpDir, archiveName);
  const extractDir = path.join(tmpDir, "extract");
  fs.mkdirSync(extractDir, { recursive: true });

  try {
    console.log(`Downloading ${archiveUrl}`);
    await downloadFile(archiveUrl, archivePath);
    await verifyChecksumIfAvailable(checksumsUrl, archivePath, archiveName);
    extractArchive(archivePath, extractDir, target.archiveType);

    const extractedBinary = findFile(extractDir, binaryName);
    if (!extractedBinary) {
      throw new Error(`archive ${archiveName} did not contain ${binaryName}`);
    }

    const destination = path.join(BIN_DIR, binaryName);
    fs.copyFileSync(extractedBinary, destination);

    if (target.platform !== "windows") {
      fs.chmodSync(destination, 0o755);
    }

    console.log(`Installed ${binaryName} to ${destination}`);
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  }
}

function installLocalBinary(sourcePath, binaryName, platform) {
  if (!fs.existsSync(sourcePath)) {
    throw new Error(`CFGFC_BINARY_PATH does not exist: ${sourcePath}`);
  }

  fs.mkdirSync(BIN_DIR, { recursive: true });
  const destination = path.join(BIN_DIR, binaryName);
  fs.copyFileSync(sourcePath, destination);

  if (platform !== "windows") {
    fs.chmodSync(destination, 0o755);
  }

  console.log(`Installed local ${binaryName} from ${sourcePath}`);
}

function resolveTarget(platform, arch) {
  const platformMap = {
    darwin: "darwin",
    linux: "linux",
    win32: "windows",
  };

  const archMap = {
    x64: "amd64",
    arm64: "arm64",
  };

  const goos = platformMap[platform];
  const goarch = archMap[arch];

  if (!goos || !goarch) {
    throw new Error(
      `unsupported platform tuple platform=${platform} arch=${arch}; supported platforms are darwin/linux/win32 on x64/arm64`,
    );
  }

  return {
    platform: goos,
    arch: goarch,
    archiveType: goos === "windows" ? "zip" : "tar.gz",
  };
}

function archiveFileName(target) {
  const extension = target.archiveType === "zip" ? "zip" : "tar.gz";
  return `${BINARY_BASE}_${VERSION}_${target.platform}_${target.arch}.${extension}`;
}

function downloadFile(url, destination) {
  return new Promise((resolve, reject) => {
    const request = https.get(url, { headers: { "User-Agent": "cfgfc-npm-installer" } }, (response) => {
      if ([301, 302, 303, 307, 308].includes(response.statusCode)) {
        response.resume();
        if (!response.headers.location) {
          reject(new Error(`redirect from ${url} did not include a location header`));
          return;
        }
        downloadFile(response.headers.location, destination).then(resolve, reject);
        return;
      }

      if (response.statusCode !== 200) {
        response.resume();
        reject(new Error(`failed to download ${url}: HTTP ${response.statusCode}`));
        return;
      }

      const file = fs.createWriteStream(destination);
      response.pipe(file);
      file.on("finish", () => file.close(resolve));
      file.on("error", reject);
    });

    request.on("error", (error) => reject(new Error(`failed to download ${url}: ${error.message}`)));
  });
}

async function verifyChecksumIfAvailable(checksumsUrl, archivePath, archiveName) {
  const checksumsPath = path.join(path.dirname(archivePath), "checksums.txt");

  try {
    await downloadFile(checksumsUrl, checksumsPath);
  } catch (error) {
    console.warn(`Warning: checksum verification skipped: ${error.message}`);
    return;
  }

  const checksums = fs.readFileSync(checksumsPath, "utf8");
  const expected = parseChecksum(checksums, archiveName);
  if (!expected) {
    console.warn(`Warning: checksum verification skipped: ${archiveName} not listed in checksums.txt`);
    return;
  }

  const actual = sha256File(archivePath);
  if (actual !== expected) {
    throw new Error(`checksum mismatch for ${archiveName}: expected ${expected}, got ${actual}`);
  }
}

function parseChecksum(checksums, archiveName) {
  for (const line of checksums.split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed) {
      continue;
    }

    const parts = trimmed.split(/\s+/);
    if (parts.length >= 2 && path.basename(parts[parts.length - 1]) === archiveName) {
      return parts[0].toLowerCase();
    }
  }
  return "";
}

function sha256File(filePath) {
  const hash = crypto.createHash("sha256");
  hash.update(fs.readFileSync(filePath));
  return hash.digest("hex");
}

function extractArchive(archivePath, destination, archiveType) {
  try {
    if (archiveType === "zip") {
      extractZip(archivePath, destination);
      return;
    }

    execFileSync("tar", ["-xzf", archivePath, "-C", destination], { stdio: "pipe" });
  } catch (error) {
    throw new Error(`failed to extract ${archivePath}: ${formatExecError(error)}`);
  }
}

function extractZip(archivePath, destination) {
  try {
    execFileSync("unzip", ["-q", archivePath, "-d", destination], { stdio: "pipe" });
  } catch (unzipError) {
    if (process.platform !== "win32") {
      throw unzipError;
    }

    execFileSync(
      "powershell.exe",
      [
        "-NoProfile",
        "-Command",
        "Expand-Archive",
        "-LiteralPath",
        archivePath,
        "-DestinationPath",
        destination,
        "-Force",
      ],
      { stdio: "pipe" },
    );
  }
}

function formatExecError(error) {
  const stderr = error.stderr ? error.stderr.toString().trim() : "";
  return stderr || error.message;
}

function findFile(root, fileName) {
  const entries = fs.readdirSync(root, { withFileTypes: true });
  for (const entry of entries) {
    const entryPath = path.join(root, entry.name);
    if (entry.isFile() && entry.name === fileName) {
      return entryPath;
    }
    if (entry.isDirectory()) {
      const found = findFile(entryPath, fileName);
      if (found) {
        return found;
      }
    }
  }
  return "";
}
