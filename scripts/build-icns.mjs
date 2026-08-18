import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";

const [iconsetPath, outputPath] = process.argv.slice(2);
if (!iconsetPath || !outputPath) {
  throw new Error("Usage: node scripts/build-icns.mjs <iconset-directory> <output.icns>");
}

const images = [
  ["icp4", "icon_16x16.png", 16],
  ["icp5", "icon_32x32.png", 32],
  ["icp6", "icon_32x32@2x.png", 64],
  ["ic07", "icon_128x128.png", 128],
  ["ic08", "icon_256x256.png", 256],
  ["ic09", "icon_512x512.png", 512],
  ["ic10", "icon_512x512@2x.png", 1024]
];

function pngSize(buffer) {
  const signature = buffer.subarray(0, 8).toString("hex");
  if (signature !== "89504e470d0a1a0a") throw new Error("ICNS inputs must be PNG files");
  return [buffer.readUInt32BE(16), buffer.readUInt32BE(20)];
}

const chunks = [];
for (const [type, filename, expectedSize] of images) {
  const png = await readFile(path.join(iconsetPath, filename));
  const [width, height] = pngSize(png);
  if (width !== expectedSize || height !== expectedSize) {
    throw new Error(`${filename} must be ${expectedSize}x${expectedSize}; received ${width}x${height}`);
  }
  const chunk = Buffer.allocUnsafe(8 + png.length);
  chunk.write(type, 0, 4, "ascii");
  chunk.writeUInt32BE(chunk.length, 4);
  png.copy(chunk, 8);
  chunks.push(chunk);
}

const totalSize = 8 + chunks.reduce((sum, chunk) => sum + chunk.length, 0);
const header = Buffer.allocUnsafe(8);
header.write("icns", 0, 4, "ascii");
header.writeUInt32BE(totalSize, 4);
await writeFile(outputPath, Buffer.concat([header, ...chunks], totalSize));
