package com.gomelo;

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.util.zip.GZIPInputStream;
import java.util.zip.GZIPOutputStream;

public class CompressionUtil {

    public enum CompressionType {
        None(0),
        Gzip(1),
        Zlib(2);

        private final int value;
        CompressionType(int value) { this.value = value; }
        public int getValue() { return value; }
    }

    public static class CompressedData {
        public final CompressionType type;
        public final byte[] data;

        public CompressedData(CompressionType type, byte[] data) {
            this.type = type;
            this.data = data;
        }
    }

    public static CompressedData compressGzip(byte[] data) {
        try {
            ByteArrayOutputStream bos = new ByteArrayOutputStream(data.length);
            GZIPOutputStream gzip = new GZIPOutputStream(bos);
            gzip.write(data);
            gzip.close();
            return new CompressedData(CompressionType.Gzip, bos.toByteArray());
        } catch (IOException e) {
            return null;
        }
    }

    public static byte[] decompress(byte[] data) {
        if (data == null || data.length < 2) return data;

        int type = data[0] & 0xFF;
        if (type == CompressionType.None.getValue()) {
            byte[] result = new byte[data.length - 1];
            System.arraycopy(data, 1, result, 0, result.length);
            return result;
        }

        byte[] compressed = new byte[data.length - 1];
        System.arraycopy(data, 1, compressed, 0, compressed.length);

        try {
            if (type == CompressionType.Gzip.getValue()) {
                ByteArrayInputStream bis = new ByteArrayInputStream(compressed);
                GZIPInputStream gzip = new GZIPInputStream(bis);
                ByteArrayOutputStream bos = new ByteArrayOutputStream();
                byte[] buffer = new byte[256];
                int len;
                while ((len = gzip.read(buffer)) != -1) {
                    bos.write(buffer, 0, len);
                }
                return bos.toByteArray();
            }
        } catch (IOException e) {
            return compressed;
        }
        return compressed;
    }

    public static boolean shouldCompress(int dataLength, int threshold) {
        return dataLength >= threshold;
    }
}