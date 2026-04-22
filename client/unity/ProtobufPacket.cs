using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text;
using Google.Protobuf;

namespace Gomelo.Network
{
    public class ProtobufPacket
    {
        public MessageType Type { get; set; }
        public string Route { get; set; }
        public ulong Seq { get; set; }
        public IMessage Body { get; set; }

        public static byte[] Encode(MessageType type, string route, ulong seq, IMessage body)
        {
            int routeId = RouteManager.GetRouteId(route);
            byte[] routeBytes = Encoding.UTF8.GetBytes(route);

            int headerSize = 1 + 1 + 8;
            int routePartSize;

            if (routeId > 0)
            {
                routePartSize = 3;
                headerSize += 2;
            }
            else
            {
                routePartSize = routeBytes.Length + 1;
                headerSize += routeBytes.Length + 1;
            }

            byte[] bodyBytes = body?.ToByteArray() ?? Array.Empty<byte>();

            int totalSize = headerSize + bodyBytes.Length;
            byte[] buffer = new byte[totalSize];
            int offset = 0;

            buffer[offset] = (byte)type;
            offset += 1;

            if (routeId > 0)
            {
                buffer[offset] = (byte)RouteFlag.RouteId;
                offset += 1;
                buffer[offset] = (byte)((routeId >> 8) & 0xFF);
                buffer[offset + 1] = (byte)(routeId & 0xFF);
                offset += 2;
            }
            else
            {
                buffer[offset] = (byte)RouteFlag.RouteString;
                offset += 1;
                Array.Copy(routeBytes, 0, buffer, offset, routeBytes.Length);
                offset += routeBytes.Length;
                buffer[offset] = 0;
                offset += 1;
            }

            byte[] seqBytes = BitConverter.GetBytes(seq);
            if (BitConverter.IsLittleEndian)
                Array.Reverse(seqBytes);
            Array.Copy(seqBytes, 0, buffer, offset, 8);
            offset += 8;

            Array.Copy(bodyBytes, 0, buffer, offset, bodyBytes.Length);

            return buffer;
        }

        public static ProtobufPacket Decode(byte[] data, IReadOnlyDictionary<string, Func<IMessage>> typeRegistry)
        {
            if (data.Length < 10)
                throw new Exception("Packet too short");

            var packet = new ProtobufPacket
            {
                Type = (MessageType)data[0]
            };

            int offset = 1;
            var flag = (RouteFlag)data[offset];
            offset += 1;

            if (flag == RouteFlag.RouteId)
            {
                if (data.Length < offset + 2)
                    throw new Exception("Invalid route id");
                int routeId = (data[offset] << 8) | data[offset + 1];
                offset += 2;
                packet.Route = RouteManager.GetRoute(routeId);
            }
            else if (flag == RouteFlag.RouteString)
            {
                int start = offset;
                while (offset < data.Length && data[offset] != 0)
                    offset++;
                packet.Route = Encoding.UTF8.GetString(data, start, offset - start);
                offset++;
            }

            if (data.Length < offset + 8)
                throw new Exception("Invalid seq");

            byte[] seqBytes = new byte[8];
            Array.Copy(data, offset, seqBytes, 0, 8);
            if (BitConverter.IsLittleEndian)
                Array.Reverse(seqBytes);
            packet.Seq = BitConverter.ToUInt64(seqBytes, 0);
            offset += 8;

            if (offset < data.Length && !string.IsNullOrEmpty(packet.Route))
            {
                byte[] bodyBytes = new byte[data.Length - offset];
                Array.Copy(data, offset, bodyBytes, 0, bodyBytes.Length);

                if (typeRegistry != null && typeRegistry.TryGetValue(packet.Route, out var factory))
                {
                    var message = factory();
                    message.MergeFrom(bodyBytes);
                    packet.Body = message;
                }
            }

            return packet;
        }
    }

    public static class ProtobufRouteRegistry
    {
        private static readonly Dictionary<string, int> _routeToId = new();
        private static readonly Dictionary<int, string> _idToRoute = new();
        private static readonly Dictionary<string, Func<IMessage>> _types = new();
        private static int _nextId = 0;

        public static void RegisterRoute(string route, int id)
        {
            _routeToId[route] = id;
            _idToRoute[id] = route;
        }

        public static void RegisterType<T>(string route, int id) where T : IMessage, new()
        {
            RegisterRoute(route, id);
            _types[route] = () => new T();
        }

        public static int GetRouteId(string route)
        {
            return _routeToId.TryGetValue(route, out int id) ? id : 0;
        }

        public static string GetRoute(int id)
        {
            return _idToRoute.TryGetValue(id, out string route) ? route : "";
        }

        public static bool TryGetMessage(string route, byte[] data, out IMessage message)
        {
            message = null;
            if (_types.TryGetValue(route, out var factory))
            {
                message = factory();
                message.MergeFrom(data);
                return true;
            }
            return false;
        }
    }
}