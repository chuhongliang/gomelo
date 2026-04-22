/**
 * Gomelo Compression Utility for Godot 4.x
 */

enum CompressionType {
    NONE = 0,
    GZIP = 1,
    ZLIB = 2
}

class CompressedData:
    var type: int
    var data: PackedByteArray

    func _init(t: int, d: PackedByteArray):
        type = t
        data = d

static func compress_gzip(data: PackedByteArray) -> CompressedData:
    var buffer := data
    var result := PackedByteArray()
    # Godot 4 doesn't have built-in gzip, would need external plugin
    # This is a placeholder - actual implementation requires GDExtension
    return CompressedData.new(CompressionType.GZIP, buffer)

static func decompress(data: PackedByteArray) -> PackedByteArray:
    if data.size() < 2:
        return data

    var compression_type := data[0]
    var compressed := data.slice(1)

    if compression_type == CompressionType.NONE:
        return compressed

    # Godot 4 doesn't have built-in decompression
    # Would need external plugin for actual gzip/zlib support
    return compressed

static func should_compress(data_length: int, threshold: int = 512) -> bool:
    return data_length >= threshold