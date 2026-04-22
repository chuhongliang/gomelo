/**
 * Gomelo Compression Utility
 * Provides gzip/zlib compression for message bodies
 */

export enum CompressionType {
    None = 0,
    Gzip = 1,
    Zlib = 2
}

export interface CompressedMessage {
    compressionType: CompressionType;
    originalSize: number;
    compressedData: ArrayBuffer;
}

export class CompressionUtil {

    private static encoder = new TextEncoder();
    private static decoder = new TextDecoder();

    static compressGzip(data: ArrayBuffer): CompressedMessage {
        const originalSize = data.byteLength;
        const compressed = pako.gzip(data);
        return {
            compressionType: CompressionType.Gzip,
            originalSize,
            compressedData: compressed.buffer
        };
    }

    static compressZlib(data: ArrayBuffer): CompressedMessage {
        const originalSize = data.byteLength;
        const compressed = pako.deflate(data);
        return {
            compressionType: CompressionType.Zlib,
            originalSize,
            compressedData: compressed.buffer
        };
    }

    static decompress(message: CompressedMessage): ArrayBuffer {
        if (message.compressionType === CompressionType.Gzip) {
            return pako.ungzip(message.compressedData);
        } else if (message.compressionType === CompressionType.Zlib) {
            return pako.inflate(message.compressedData);
        }
        return message.compressedData;
    }

    static shouldCompress(dataLength: number, threshold: number = 512): boolean {
        return dataLength >= threshold;
    }
}

export class GomeloClient {

    public enableCompression: boolean = false;
    public compressionThreshold: number = 512;

    private _compress(data: ArrayBuffer): ArrayBuffer {
        if (!this.enableCompression || data.byteLength < this.compressionThreshold) {
            return data;
        }
        const compressed = CompressionUtil.compressGzip(data);
        const header = new ArrayBuffer(2);
        const headerView = new DataView(header);
        headerView.setUint8(0, CompressionType.Gzip);
        headerView.setUint8(1, 0);

        const combined = new ArrayBuffer(2 + compressed.compressedData.byteLength);
        const combinedView = new DataView(combined);
        const combinedBytes = new Uint8Array(combined);
        combinedBytes.set(new Uint8Array(header), 0);
        combinedBytes.set(new Uint8Array(compressed.compressedData), 2);
        return combined;
    }

    private _decompress(data: ArrayBuffer): ArrayBuffer {
        if (data.byteLength < 2) return data;
        const view = new DataView(data);
        const type = view.getUint8(0);
        if (type === CompressionType.None) {
            return data.slice(2);
        }
        const compressed = {
            compressionType: type,
            originalSize: 0,
            compressedData: data.slice(2)
        };
        return CompressionUtil.decompress(compressed);
    }
}