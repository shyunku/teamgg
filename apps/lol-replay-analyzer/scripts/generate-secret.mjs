import { randomBytes } from "node:crypto";

const rawByteLength = process.argv[2] ?? "48";
const byteLength = Number(rawByteLength);

if (!Number.isSafeInteger(byteLength) || byteLength < 32 || byteLength > 1024) {
  console.error("Byte length must be an integer between 32 and 1024.");
  process.exit(1);
}

process.stdout.write(`${randomBytes(byteLength).toString("base64url")}\n`);
