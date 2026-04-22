using System;
using System.IO;
using System.IO.Compression;

namespace Gomelo.Network
{
    public enum CompressionType : byte
    {
        None = 0,
        Gzip = 1,
        Zlib = 2
    }

    public class CompressedData
    {
        public CompressionType Type { get; private set; }
        public byte[] Data { get; private set; }

        public CompressedData(CompressionType type, byte[] data)
        {
            Type = type;
            Data = data;
        }
    }

    public static class CompressionUtil
    {
        public static CompressedData CompressGzip(byte[] data)
        {
            try
            {
                using var output = new MemoryStream();
                using (var gzip = new GZipStream(output, CompressionLevel.Fastest))
                {
                    gzip.Write(data, 0, data.Length);
                }
                return new CompressedData(CompressionType.Gzip, output.ToArray());
            }
            catch
            {
                return null;
            }
        }

        public static byte[] Decompress(byte[] data)
        {
            if (data == null || data.Length < 2)
                return data;

            CompressionType type = (CompressionType)data[0];
            if (type == CompressionType.None)
                return data.Length > 1 ? data.SubArray(1, data.Length - 1) : Array.Empty<byte>();

            byte[] compressed = new byte[data.Length - 1];
            Array.Copy(data, 1, compressed, 0, compressed.Length);

            try
            {
                if (type == CompressionType.Gzip)
                {
                    using var input = new MemoryStream(compressed);
                    using var gzip = new GZipStream(input, CompressionMode.Decompress);
                    using var output = new MemoryStream();
                    gzip.CopyTo(output);
                    return output.ToArray();
                }
            }
            catch { }

            return compressed;
        }

        public static bool ShouldCompress(int dataLength, int threshold = 512)
        {
            return dataLength >= threshold;
        }
    }
}

public static class ByteArrayExtensions
{
    public static byte[] SubArray(this byte[] data, int offset, int length)
    {
        byte[] result = new byte[length];
        Array.Copy(data, offset, result, 0, length);
        return result;
    }
}