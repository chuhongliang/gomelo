using System;

namespace Gomelo.Network
{
    public enum MessageType
    {
        Request = 1,
        Response = 2,
        Notify = 3,
        Error = 4
    }

    public enum RouteFlag
    {
        RouteId = 0x01,
        RouteString = 0x00
    }

    public enum ProtocolType
    {
        WebSocket,
        TCP,
        UDP
    }

    public static class Protocol
    {
        public const int DefaultPort = 3010;
        public const int DefaultTimeout = 5000;
        public const int DefaultHeartbeatInterval = 30000;
    }
}