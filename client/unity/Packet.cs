using System;
using System.Collections.Generic;
using System.Text;
using System.IO;

namespace Gomelo.Network
{
    public class Packet
    {
        public MessageType Type { get; set; }
        public string Route { get; set; }
        public uint Seq { get; set; }
        public object Body { get; set; }

        public Packet() { }

        public Packet(MessageType type, string route, uint seq, object body)
        {
            Type = type;
            Route = route;
            Seq = seq;
            Body = body;
        }

        public static byte[] Encode(MessageType type, string route, uint seq, object body)
        {
            var bodyJson = body != null ? SimpleJson.Serialize(body) : "{}";
            var bodyBytes = Encoding.UTF8.GetBytes(bodyJson);

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

        public static Packet Decode(byte[] data)
        {
            if (data.Length < 10)
                throw new Exception("Packet too short");

            var packet = new Packet
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
            packet.Seq = BitConverter.ToUInt32(seqBytes, 0);
            offset += 8;

            if (offset < data.Length)
            {
                byte[] bodyBytes = new byte[data.Length - offset];
                Array.Copy(data, offset, bodyBytes, 0, bodyBytes.Length);
                string jsonStr = Encoding.UTF8.GetString(bodyBytes);
                if (!string.IsNullOrEmpty(jsonStr))
                {
                    packet.Body = SimpleJson.Deserialize(jsonStr);
                }
            }

            return packet;
        }
    }

    public static class RouteManager
    {
        private static readonly Dictionary<string, int> _routeToId = new();
        private static readonly Dictionary<int, string> _idToRoute = new();
        private static int _nextId = 0;

        public static void RegisterRoute(string route, int id)
        {
            _routeToId[route] = id;
            _idToRoute[id] = route;
        }

        public static int GetRouteId(string route)
        {
            return _routeToId.TryGetValue(route, out int id) ? id : 0;
        }

        public static string GetRoute(int id)
        {
            return _idToRoute.TryGetValue(id, out string route) ? route : "";
        }

        public static void Clear()
        {
            _routeToId.Clear();
            _idToRoute.Clear();
            _nextId = 0;
        }
    }

    public static class SimpleJson
    {
        public static string Serialize(object obj)
        {
            if (obj == null) return "null";
            if (obj is Dictionary<string, object> dict)
                return SerializeDict(dict);
            if (obj is List<object> list)
                return SerializeList(list);
            if (obj is string s)
                return $"\"{EscapeString(s)}\"";
            if (obj is bool b)
                return b ? "true" : "false";
            if (obj is int || obj is long || obj is float || obj is double)
                return obj.ToString().Replace(",", ".");
            return $"\"{obj}\"";
        }

        private static string SerializeDict(Dictionary<string, object> dict)
        {
            var parts = new List<string>();
            foreach (var kv in dict)
            {
                parts.Add($"\"{kv.Key}\":{Serialize(kv.Value)}");
            }
            return "{" + string.Join(",", parts) + "}";
        }

        private static string SerializeList(List<object> list)
        {
            var parts = new List<string>();
            foreach (var item in list)
            {
                parts.Add(Serialize(item));
            }
            return "[" + string.Join(",", parts) + "]";
        }

        private static string EscapeString(string s)
        {
            return s.Replace("\\", "\\\\").Replace("\"", "\\\"");
        }

        public static object Deserialize(string json)
        {
            json = json.Trim();
            if (json.StartsWith("{"))
                return DeserializeDict(json);
            if (json.StartsWith("["))
                return DeserializeList(json);
            if (json == "null")
                return null;
            if (json == "true")
                return true;
            if (json == "false")
                return false;
            if (json.StartsWith("\""))
                return DeserializeString(json);
            return ParseNumber(json);
        }

        private static Dictionary<string, object> DeserializeDict(string json)
        {
            var result = new Dictionary<string, object>();
            json = json.Trim().TrimStart('{').TrimEnd('}');
            if (string.IsNullOrEmpty(json)) return result;

            var pairs = SplitTokens(json);
            foreach (var pair in pairs)
            {
                var colonIdx = pair.IndexOf(':');
                var key = DeserializeString(pair.Substring(0, colonIdx).Trim()).ToString();
                var value = Deserialize(pair.Substring(colonIdx + 1).Trim());
                result[key] = value;
            }
            return result;
        }

        private static List<object> DeserializeList(string json)
        {
            var result = new List<object>();
            json = json.Trim().TrimStart('[').TrimEnd(']');
            if (string.IsNullOrEmpty(json)) return result;

            var items = SplitTokens(json);
            foreach (var item in items)
            {
                result.Add(Deserialize(item.Trim()));
            }
            return result;
        }

        private static string DeserializeString(string json)
        {
            if (!json.StartsWith("\"") || !json.EndsWith("\""))
                return json;
            json = json.Substring(1, json.Length - 2);
            return json.Replace("\\\"", "\"").Replace("\\\\", "\\");
        }

        private static object ParseNumber(string json)
        {
            if (json.Contains("."))
            {
                if (double.TryParse(json, System.Globalization.NumberStyles.Float,
                    System.Globalization.CultureInfo.InvariantCulture, out double d))
                    return d;
            }
            else
            {
                if (long.TryParse(json, out long l))
                    return l;
            }
            return json;
        }

        private static List<string> SplitTokens(string json)
        {
            var tokens = new List<string>();
            int depth = 0;
            int start = 0;
            bool inString = false;

            for (int i = 0; i < json.Length; i++)
            {
                char c = json[i];
                if (c == '"' && (i == 0 || json[i - 1] != '\\'))
                    inString = !inString;
                else if (!inString)
                {
                    if (c == '{' || c == '[')
                        depth++;
                    else if (c == '}' || c == ']')
                        depth--;
                    else if (c == ',' && depth == 0)
                    {
                        tokens.Add(json.Substring(start, i - start));
                        start = i + 1;
                    }
                }
            }
            if (start < json.Length)
                tokens.Add(json.Substring(start));
            return tokens;
        }
    }
}