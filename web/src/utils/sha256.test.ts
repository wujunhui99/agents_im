import { describe, expect, it } from 'vitest';
import { digestSha256 } from './sha256';

// 已知向量：空输入与 "abc" 的 SHA-256（FIPS 180-4 附录），固定 hex + base64。
const EMPTY_HEX = 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855';
const EMPTY_BASE64 = '47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU=';
const ABC_HEX = 'ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad';

describe('digestSha256', () => {
  it('matches the known SHA-256 vector for empty input as both hex and base64', async () => {
    const digest = await digestSha256(new Uint8Array());

    expect(digest.hex).toBe(EMPTY_HEX);
    expect(digest.base64).toBe(EMPTY_BASE64);
  });

  it('hashes a Blob body to the known "abc" vector', async () => {
    const digest = await digestSha256(new Blob(['abc']));

    expect(digest.hex).toBe(ABC_HEX);
  });

  it('produces a lowercase 64-char hex and round-trips base64 to the same digest bytes', async () => {
    const digest = await digestSha256(new TextEncoder().encode('agents_im'));

    expect(digest.hex).toMatch(/^[0-9a-f]{64}$/);
    const fromBase64 = Uint8Array.from(atob(digest.base64), (c) => c.charCodeAt(0));
    const fromHex = digest.hex.match(/.{2}/g)!.map((byte) => parseInt(byte, 16));
    expect(Array.from(fromBase64)).toEqual(fromHex);
  });

  it('hashes ArrayBuffer, Uint8Array, and Blob of the same bytes identically', async () => {
    const bytes = new TextEncoder().encode('same-bytes');
    const fromArray = await digestSha256(bytes);
    const fromBuffer = await digestSha256(bytes.buffer.slice(0));
    const fromBlob = await digestSha256(new Blob([bytes]));

    expect(fromBuffer.hex).toBe(fromArray.hex);
    expect(fromBlob.hex).toBe(fromArray.hex);
  });
});
