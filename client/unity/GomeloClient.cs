using System;
using System.Collections.Generic;
using System.Net.Sockets;
using System.Net.WebSockets;
using System.Text;
using System.Threading;
using UnityEngine;
using System.Threading.Tasks;
using System.Net;

namespace Gomelo
{
    public class GomeloClient : MonoBehaviour
    {
        public string Host = "localhost";
        public int Port = Network.Protocol.DefaultPort;
        public Network.ProtocolType Protocol = Network.ProtocolType.WebSocket;
        public int Timeout = Network.Protocol.DefaultTimeout;
        public int HeartbeatInterval = Network.Protocol.DefaultHeartbeatInterval;

        public event Action OnConnected;
        public event Action OnDisconnected;
        public event Action<string> OnError;
        public event Action<ulong, object> OnResponse;
        public event Action<string, object> OnNotify;

        private ClientWebSocket _ws;
        private TcpClient _tcpClient;
        private UdpClient _udpClient;
        private NetworkStream _tcpStream;
        private IPEndPoint _udpEndPoint;
        private bool _connected;
        private bool _closed;
        private ulong _seq;
        private readonly Dictionary<ulong, Action<object>> _callbacks = new();
        private readonly Dictionary<ulong, Action<object>> _errorCallbacks = new();
        private readonly Dictionary<string, List<Action<object>>> _eventHandlers = new();
        private CancellationTokenSource _cts;
        private CancellationToken _ct;
        private byte[] _receiveBuffer = new byte[8192];
        private List<byte> _messageBuffer = new List<byte>();
        private bool _receiving;
        private Thread _tcpReadThread;
        private Thread _udpReadThread;
        private bool _isRunning;
        private ulong _nextSeq => ++_seq == 0 ? ++_seq : _seq;
        private bool _schemaReceived;
        private readonly Dictionary<int, string> _routeIdToCodec = new();
        private readonly Dictionary<int, string> _routeIdToTypeUrl = new();
        private ProtobufPacket _protobufCodec = new ProtobufPacket();

        async void Start()
        {
            _cts = new CancellationTokenSource();
            _ct = _cts.Token;
        }

        void Update()
        {
            if (Protocol == Network.ProtocolType.WebSocket)
            {
                if (_ws != null && _ws.State == WebSocketState.Open && !_receiving)
                {
                    ReceiveWSMessageAsync();
                }
            }
        }

        void OnDestroy()
        {
            Disconnect();
        }

        public async void Connect(string host = "", int port = -1, Network.ProtocolType protocol = Network.ProtocolType.WebSocket)
        {
            if (!string.IsNullOrEmpty(host))
                Host = host;
            if (port > 0)
                Port = port;
            Protocol = protocol;

            _closed = false;
            _connected = false;

            try
            {
                switch (Protocol)
                {
                    case Network.ProtocolType.TCP:
                        ConnectTCP();
                        break;
                    case Network.ProtocolType.UDP:
                        ConnectUDP();
                        break;
                    case Network.ProtocolType.WebSocket:
                    default:
                        await ConnectWSAsync();
                        break;
                }
            }
            catch (Exception e)
            {
                _connected = false;
                OnError?.Invoke(e.Message);
            }
        }

        private async System.Threading.Tasks.Task ConnectWSAsync()
        {
            if (_ws != null)
            {
                try { _ws.CloseAsync(WebSocketCloseStatus.NormalClosure, "", CancellationToken.None).Wait(1000); } catch { }
            }

            _ws = new ClientWebSocket();
            _ws.Options.SetBuffer(8192, 8192);

            await _ws.ConnectAsync(new Uri($"ws://{Host}:{Port}"), _ct);
            _connected = true;
            OnConnected?.Invoke();
            _ = SendHeartbeatAsync();
        }

        private void ConnectTCP()
        {
            _tcpClient = new TcpClient();
            _tcpClient.Connect(Host, Port);
            _tcpClient.NoDelay = true;
            _tcpStream = _tcpClient.GetStream();
            _isRunning = true;

            _tcpReadThread = new Thread(TCPReadLoop);
            _tcpReadThread.IsBackground = true;
            _tcpReadThread.Start();

            _connected = true;
            OnConnected?.Invoke();
            _ = SendHeartbeatAsync();
        }

        private void TCPReadLoop()
        {
            byte[] buffer = new byte[65536];
            List<byte> dataBuffer = new List<byte>();

            try
            {
                while (_isRunning && _tcpClient != null && _tcpClient.Connected)
                {
                    if (_tcpStream.DataAvailable)
                    {
                        int read = _tcpStream.Read(buffer, 0, buffer.Length);
                        if (read == 0) break;

                        for (int i = 0; i < read; i++)
                            dataBuffer.Add(buffer[i]);

                        while (dataBuffer.Count >= 4)
                        {
                            int length = (dataBuffer[0] << 24) | (dataBuffer[1] << 16) | (dataBuffer[2] << 8) | dataBuffer[3];
                            int totalLen = 4 + length;

                            if (dataBuffer.Count < totalLen)
                                break;

                            byte[] packet = new byte[length];
                            dataBuffer.CopyTo(0, packet, 0, length);
                            dataBuffer.RemoveRange(0, totalLen);

                            HandlePacket(packet);
                        }
                    }
                    Thread.Sleep(10);
                }
            }
            catch (Exception)
            {
            }

            if (_isRunning && !_closed)
            {
                _connected = false;
                OnDisconnected?.Invoke();
            }
        }

        private void ConnectUDP()
        {
            _udpClient = new UdpClient();
            _udpEndPoint = new IPEndPoint(IPAddress.Parse(Host), Port);
            _isRunning = true;

            _udpReadThread = new Thread(UDPReadLoop);
            _udpReadThread.IsBackground = true;
            _udpReadThread.Start();

            _connected = true;
            OnConnected?.Invoke();
        }

        private void UDPReadLoop()
        {
            try
            {
                while (_isRunning && _udpClient != null)
                {
                    if (_udpClient.Available > 0)
                    {
                        var result = _udpClient.Receive(ref _udpEndPoint);
                        HandlePacket(result);
                    }
                    Thread.Sleep(10);
                }
            }
            catch (Exception)
            {
            }

            if (_isRunning && !_closed)
            {
                _connected = false;
                OnDisconnected?.Invoke();
            }
        }

        private async void ReceiveWSMessageAsync()
        {
            if (_ws == null || _ws.State != WebSocketState.Open || _receiving) return;

            _receiving = true;
            try
            {
                var result = await _ws.ReceiveAsync(new ArraySegment<byte>(_receiveBuffer), _ct);
                if (result.MessageType == WebSocketMessageType.Close)
                {
                    _connected = false;
                    OnDisconnected?.Invoke();
                    if (!_closed) _ = TryReconnectAsync();
                }
                else if (result.MessageType == WebSocketMessageType.Binary)
                {
                    for (int i = 0; i < result.Count; i++)
                        _messageBuffer.Add(_receiveBuffer[i]);

                    if (result.EndOfMessage)
                    {
                        var data = _messageBuffer.ToArray();
                        _messageBuffer.Clear();
                        HandlePacket(data);
                    }
                }
            }
            catch (Exception) { }
            finally { _receiving = false; }
        }

        private async System.Threading.Tasks.Task SendHeartbeatAsync()
        {
            while (_connected && !_closed && !_ct.IsCancellationRequested)
            {
                await System.Threading.Tasks.Task.Delay(HeartbeatInterval, _ct).ContinueWith(t => { }, System.Threading.Tasks.TaskScheduler.Ordinal);
                if (_connected && !_closed)
                    Notify("sys.heartbeat", new Dictionary<string, object> { { "ts", DateTimeOffset.UtcNow.ToUnixTimeMilliseconds() } });
            }
        }

        private async System.Threading.Tasks.Task TryReconnectAsync()
        {
            if (_closed || Protocol == Network.ProtocolType.UDP) return;
            for (int i = 0; i < 5 && !_closed; i++)
            {
                await System.Threading.Tasks.Task.Delay(3000 * (i + 1), _ct).ContinueWith(t => { }, System.Threading.Tasks.TaskScheduler.Ordinal);
                if (_closed) return;
                Connect(Host, Port, Protocol);
                if (_connected) return;
            }
        }

        public void Disconnect()
        {
            _closed = true;
            _connected = false;
            _isRunning = false;
            _cts?.Cancel();

            if (_ws != null)
            {
                try { _ws.CloseAsync(WebSocketCloseStatus.NormalClosure, "Client disconnect", CancellationToken.None).Wait(1000); } catch { }
                _ws = null;
            }

            _tcpReadThread = null;
            if (_tcpStream != null)
            {
                try { _tcpStream.Close(); } catch { }
                _tcpStream = null;
            }
            if (_tcpClient != null)
            {
                try { _tcpClient.Close(); } catch { }
                _tcpClient = null;
            }

            if (_udpClient != null)
            {
                try { _udpClient.Close(); } catch { }
                _udpClient = null;
            }

            _clearPending("Disconnected");
        }

        public ulong Request(string route, object body, Action<object> onSuccess, Action<object> onError = null)
        {
            if (!_connected)
            {
                onError?.Invoke("Not connected");
                return 0;
            }

            ulong seq = _nextSeq;
            var data = Network.Packet.Encode(Network.MessageType.Request, route, seq, body);

            try
            {
                SendData(data);
            }
            catch (Exception e)
            {
                onError?.Invoke(e.Message);
                return 0;
            }

            _callbacks[seq] = onSuccess;
            if (onError != null) _errorCallbacks[seq] = onError;

            _ = System.Threading.Tasks.Task.Delay(Timeout).ContinueWith(_ =>
            {
                if (_callbacks.Remove(seq, out var cb)) onError?.Invoke("Request timeout");
            });

            return seq;
        }

        public async void Notify(string route, object body = null)
        {
            if (!_connected) return;
            var data = Network.Packet.Encode(Network.MessageType.Notify, route, 0, body);
            try
            {
                await System.Threading.Tasks.Task.Run(() => SendData(data));
            }
            catch { }
        }

        private void SendData(byte[] data)
        {
            switch (Protocol)
            {
                case Network.ProtocolType.TCP:
                    if (_tcpStream != null && _tcpClient.Connected)
                    {
                        byte[] lengthBytes = new byte[4];
                        lengthBytes[0] = (byte)(data.Length >> 24);
                        lengthBytes[1] = (byte)(data.Length >> 16);
                        lengthBytes[2] = (byte)(data.Length >> 8);
                        lengthBytes[3] = (byte)data.Length;
                        _tcpStream.Write(lengthBytes, 0, 4);
                        _tcpStream.Write(data, 0, data.Length);
                    }
                    break;
                case Network.ProtocolType.UDP:
                    if (_udpClient != null)
                    {
                        _udpClient.Send(data, data.Length);
                    }
                    break;
                case Network.ProtocolType.WebSocket:
                default:
                    if (_ws != null && _ws.State == WebSocketState.Open)
                    {
                        _ws.SendAsync(new ArraySegment<byte>(data), WebSocketMessageType.Binary, true, _ct).Wait();
                    }
                    break;
            }
        }

        public void On(string route, Action<object> handler)
        {
            if (!_eventHandlers.ContainsKey(route)) _eventHandlers[route] = new List<Action<object>>();
            _eventHandlers[route].Add(handler);
        }

        public void Off(string route, Action<object> handler = null)
        {
            if (handler == null) _eventHandlers.Remove(route);
            else if (_eventHandlers.ContainsKey(route)) _eventHandlers[route].Remove(handler);
        }

        public void RegisterRoute(string route, int id) => Network.RouteManager.RegisterRoute(route, id);

        public bool IsConnected => _connected;

        private void HandlePacket(byte[] data)
        {
            try
            {
                if (data.Length < 4) return;
                int bodyLen = (data[0] << 24) | (data[1] << 16) | (data[2] << 8) | data[3];
                if (bodyLen > 64 * 1024 || bodyLen == 0) return;
                if (4 + bodyLen > data.Length) return;

                string bodyStr = Encoding.UTF8.GetString(data, 4, bodyLen);
                var dict = SimpleJson.Deserialize<Dictionary<string, object>>(bodyStr);
                if (dict != null && dict.TryGetValue("type", out var typeVal) && typeVal?.ToString() == "schema")
                {
                    HandleSchema(dict);
                    return;
                }

                var packet = Network.Packet.Decode(data);
                switch (packet.Type)
                {
                    case Network.MessageType.Response:
                        if (_callbacks.Remove(packet.Seq, out var cb))
                        {
                            _errorCallbacks.Remove(packet.Seq);
                            cb?.Invoke(packet.Body);
                        }
                        OnResponse?.Invoke(packet.Seq, packet.Body);
                        break;
                    case Network.MessageType.Notify:
                    case Network.MessageType.Error:
                        OnNotify?.Invoke(packet.Route, packet.Body);
                        if (_eventHandlers.ContainsKey(packet.Route))
                            foreach (var h in _eventHandlers[packet.Route]) h?.Invoke(packet.Body);
                        break;
                }
            }
            catch (Exception e) { Debug.LogError($"HandlePacket error: {e.Message}"); }
        }

        private void HandleSchema(Dictionary<string, object> data)
        {
            if (!data.ContainsKey("data")) return;
            var schemaData = data["data"] as Dictionary<string, object>;
            if (schemaData == null || !schemaData.ContainsKey("routes")) return;

            var routes = schemaData["routes"] as List<object>;
            if (routes == null) return;

            foreach (var r in routes)
            {
                var route = r as Dictionary<string, object>;
                if (route == null) continue;

                string routeStr = route["route"]?.ToString();
                int id = Convert.ToInt32(route["id"]);
                string codec = route["codec"]?.ToString();
                string typeUrl = route["typeUrl"]?.ToString();

                if (string.IsNullOrEmpty(routeStr)) continue;

                Network.RouteManager.RegisterRoute(routeStr, id);
                if (!string.IsNullOrEmpty(codec))
                {
                    _routeIdToCodec[id] = codec;
                    if (!string.IsNullOrEmpty(typeUrl))
                    {
                        _routeIdToTypeUrl[id] = typeUrl;
                    }
                }
            }
            _schemaReceived = true;
        }

        private void _clearPending(string msg)
        {
            foreach (var cb in _callbacks.Values) cb?.Invoke(msg);
            _callbacks.Clear();
            _errorCallbacks.Clear();
        }
    }
}