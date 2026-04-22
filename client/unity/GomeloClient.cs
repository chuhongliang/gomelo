using System;
using System.Collections.Generic;
using System.Net.WebSockets;
using System.Text;
using System.Threading;
using UnityEngine;

namespace Gomelo
{
    public class GomeloClient : MonoBehaviour
    {
        public string Host = "localhost";
        public int Port = Network.Protocol.DefaultPort;
        public int Timeout = Network.Protocol.DefaultTimeout;
        public int HeartbeatInterval = 30000;

        public event Action OnConnected;
        public event Action OnDisconnected;
        public event Action<string> OnError;
        public event Action<ulong, object> OnResponse;
        public event Action<string, object> OnNotify;

        private ClientWebSocket _ws;
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
        private ulong _nextSeq => ++_seq == 0 ? ++_seq : _seq;

        async void Start()
        {
            _cts = new CancellationTokenSource();
            _ct = _cts.Token;
        }

        void Update()
        {
            if (_ws != null && _ws.State == WebSocketState.Open && !_receiving)
            {
                ReceiveMessageAsync();
            }
        }

        public async void Connect(string host = "", int port = -1)
        {
            if (!string.IsNullOrEmpty(host))
                Host = host;
            if (port > 0)
                Port = port;

            _closed = false;

            if (_ws != null)
            {
                try { _ws.CloseAsync(WebSocketCloseStatus.NormalClosure, "", CancellationToken.None); } catch { }
            }

            _ws = new ClientWebSocket();
            _ws.Options.SetBuffer(8192, 8192);

            try
            {
                await _ws.ConnectAsync(new Uri($"ws://{Host}:{Port}"), _ct);
                _connected = true;
                OnConnected?.Invoke();
                _ = SendHeartbeatAsync();
            }
            catch (Exception e)
            {
                _connected = false;
                OnError?.Invoke(e.Message);
            }
        }

        private async void ReceiveMessageAsync()
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

        private async void SendHeartbeatAsync()
        {
            while (_ws != null && _ws.State == WebSocketState.Open && !_closed && !_ct.IsCancellationRequested)
            {
                await Task.Delay(HeartbeatInterval, _ct).ContinueWith(t => { }, TaskScheduler.Ordinal);
                if (_ws.State == WebSocketState.Open && !_closed)
                    Notify("sys.heartbeat", new Dictionary<string, object> { { "ts", DateTimeOffset.UtcNow.ToUnixTimeMilliseconds() } });
            }
        }

        private async Task TryReconnectAsync()
        {
            if (_closed) return;
            for (int i = 0; i < 5 && !_closed; i++)
            {
                await Task.Delay(3000 * (i + 1), _ct).ContinueWith(t => { }, TaskScheduler.Ordinal);
                if (_closed) return;
                Connect(Host, Port);
                if (_connected) return;
            }
        }

        public void Disconnect()
        {
            _closed = true;
            _connected = false;
            _cts?.Cancel();
            try { _ws?.CloseAsync(WebSocketCloseStatus.NormalClosure, "Client disconnect", CancellationToken.None); } catch { }
            _ws = null;
            _clearPending("Disconnected");
        }

        public ulong Request(string route, object body, Action<object> onSuccess, Action<object> onError = null)
        {
            if (_ws == null || _ws.State != WebSocketState.Open)
            {
                onError?.Invoke("Not connected");
                return 0;
            }

            ulong seq = _nextSeq;
            var data = Network.Packet.Encode(Network.MessageType.Request, route, seq, body);
            _ws.SendAsync(new ArraySegment<byte>(data), WebSocketMessageType.Binary, true, _ct).ContinueWith(t =>
            {
                if (t.IsFaulted) onError?.Invoke(t.Exception?.Message);
            });
            _callbacks[seq] = onSuccess;
            if (onError != null) _errorCallbacks[seq] = onError;

            _ = Task.Delay(Timeout).ContinueWith(_ =>
            {
                if (_callbacks.Remove(seq, out var cb)) onError?.Invoke("Request timeout");
            });

            return seq;
        }

        public async void Notify(string route, object body = null)
        {
            if (_ws == null || _ws.State != WebSocketState.Open) return;
            var data = Network.Packet.Encode(Network.MessageType.Notify, route, 0, body);
            await _ws.SendAsync(new ArraySegment<byte>(data), WebSocketMessageType.Binary, true, _ct);
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

        public bool IsConnected => _connected && _ws?.State == WebSocketState.Open;

        void HandlePacket(byte[] data)
        {
            try
            {
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

        void _clearPending(string msg)
        {
            foreach (var cb in _callbacks.Values) cb?.Invoke(msg);
            _callbacks.Clear();
            _errorCallbacks.Clear();
        }

        void OnDestroy() => Disconnect();
    }
}