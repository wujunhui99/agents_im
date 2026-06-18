// 整文件 SHA-256 摘要（EPIC #527 §3，内容寻址）。浏览器原生 WebCrypto，无需 WASM；
// 不分片：把整文件读入一个 ArrayBuffer 后一次性摘要。返回 hex（喂 CreateUploadIntent 的
// sha256 字段，object_key=agents_im/{sha256}）与 base64（回放到 presigned PUT 的
// x-amz-checksum-sha256 头，OSS 据此校验字节完整性）。

export type Sha256Digest = {
  /** 小写 64 位 hex —— 契约里 sha256 字段、内容寻址 key 用这个。 */
  hex: string;
  /** 原始 32 字节摘要的 base64 —— x-amz-checksum-sha256 头用这个。 */
  base64: string;
};

export async function digestSha256(data: Blob | ArrayBuffer | Uint8Array): Promise<Sha256Digest> {
  const subtle = resolveSubtle();
  const view = await toLocalView(data);
  const hash = await subtle.digest('SHA-256', view);
  const bytes = new Uint8Array(hash);
  return { hex: bytesToHex(bytes), base64: bytesToBase64(bytes) };
}

function resolveSubtle(): SubtleCrypto {
  const cryptoObj = globalThis.crypto;
  if (!cryptoObj || !cryptoObj.subtle) {
    throw new Error('当前环境不支持 WebCrypto（crypto.subtle），无法计算文件校验和');
  }
  return cryptoObj.subtle;
}

// 统一归一为本 realm 构造的 Uint8Array 视图后再交给 subtle.digest。
// 用鸭子类型而非 instanceof：jsdom 与 Node 是不同 realm，跨 realm instanceof 不可靠；
// 且 Blob.arrayBuffer() 在 jsdom 下返回的 ArrayBuffer 跨 realm，Node webcrypto 的 digest
// 做 brand 校验会拒收 —— 用本地 Uint8Array 包一层即可让参数通过 TypedArray 校验。
async function toLocalView(data: Blob | ArrayBuffer | Uint8Array): Promise<Uint8Array<ArrayBuffer>> {
  const maybeBlob = data as Blob;
  if (typeof maybeBlob.arrayBuffer === 'function') {
    return new Uint8Array(await maybeBlob.arrayBuffer());
  }
  if (ArrayBuffer.isView(data)) {
    const view = data as ArrayBufferView;
    return new Uint8Array(view.buffer as ArrayBuffer, view.byteOffset, view.byteLength);
  }
  return new Uint8Array(data as ArrayBuffer);
}

function bytesToHex(bytes: Uint8Array): string {
  let hex = '';
  for (const byte of bytes) {
    hex += byte.toString(16).padStart(2, '0');
  }
  return hex;
}

function bytesToBase64(bytes: Uint8Array): string {
  let binary = '';
  for (const byte of bytes) {
    binary += String.fromCharCode(byte);
  }
  return btoa(binary);
}
