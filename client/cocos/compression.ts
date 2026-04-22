/**
 * Gomelo Compression Utility for Cocos Creator 3.x
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
            return pako.ungzip(message.compressedData) as ArrayBuffer;
        } else if (message.compressionType === CompressionType.Zlib) {
            return pako.inflate(message.compressedData) as ArrayBuffer;
        }
        return message.compressedData;
    }

    static shouldCompress(dataLength: number, threshold: number = 512): boolean {
        return dataLength >= threshold;
    }
}

export function applyCompression(
    client: GomeloClient,
    enableCompression: boolean = false,
    compressionThreshold: number = 512
): void {
    (client as any)._compress = function(data: ArrayBuffer): ArrayBuffer {
        if (!enableCompression || data.byteLength < compressionThreshold) {
            return data;
        }
        const compressed = CompressionUtil.compressGzip(data);
        const header = new Uint8Array(2);
        header[0] = CompressionType.Gzip;
        header[1] = 0;

        const combined = new Uint8Array(2 + compressed.compressedData.byteLength);
        combined.set(header, 0);
        combined.set(new Uint8Array(compressed.compressedData), 2);
        return combined.buffer;
    };

    (client as any)._decompress = function(data: ArrayBuffer): ArrayBuffer {
        if (data.byteLength < 2) return data;
        const view = new DataView(data);
        const type = view.getUint8(0);
        if (type === CompressionType.None) {
            return data.slice(1);
        }
        const compressed = {
            compressionType: type,
            originalSize: 0,
            compressedData: data.slice(1)
        };
        return CompressionUtil.decompress(compressed);
    };
}