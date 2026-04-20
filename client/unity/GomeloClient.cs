using System;
using System.Collections.Generic;
using UnityEngine;
using WebSocketSharp;

namespace Gomelo
{
    public class GomeloClient : MonoBehaviour
    {
        public string Host = "localhost";
        public int Port = Network.Protocol.DefaultPort;
        public int Timeout = Network.Protocol.DefaultTimeout;

        public event Action OnConnected;
        public event Action OnDisconnected;
        public event Action<string> OnError;
        public event Action<uint, object> OnResponse;
        public event Action<string, object> OnNotify;

        private WebSocketSharp.WebSocket _ws;
        private bool _connected;
        private uint _seq;
        private readonly Dictionary<uint, Action<object>> _callbacks = new();
        private readonly Dictionary<uint, Action<object>> _errorCallbacks = new();
        private readonly Dictionary<string, List<Action<object>>> _eventHandlers = new();

        void Update()
        {
            if (_ws != null && _ws.ReadyState == WebSocketState.Open)
            {
                byte[] data = null;
                try
                {
                    data = _ws.Recv();
                }
                catch { }

                if (data != null && data.Length > 0)
                {
                    HandlePacket(data);
                }
            }
        }

        public void Connect(string host = "", int port = -1)
        {
            if (!string.IsNullOrEmpty(host))
                Host = host;
            if (port > 0)
                Port = port;

            string url = $"ws://{Host}:{Port}";
            _ws = new WebSocketSharp.WebSocket(url);
            _ws.OnOpen += (sender, e) =>
            {
                _connected = true;
                OnConnected?.Invoke();
            };
            _ws.OnClose += (sender, e) =>
            {
                _connected = false;
                OnDisconnected?.Invoke();
            };
            _ws.OnError += (sender, e) =>
            {
                OnError?.Invoke(e.Message);
            };
            _ws.OnMessage += (sender, e) =>
            {
                if (e.Data != null && e.Data.Length > 0)
                {
                    HandlePacket(Encoding.UTF8.GetBytes(e.Data));
                }
            };
            _ws.ConnectAsync();
        }

        public void Disconnect()
        {
            if (_ws != null)
            {
                _ws.Close();
                _connected = false;
            }
        }

        public uint Request(string route, object body, Action<object> onSuccess, Action<object> onError = null)
        {
            if (_ws == null || _ws.ReadyState != WebSocketState.Open)
            {
                onError?.Invoke("Not connected");
                return 0;
            }

            uint seq = GetNextSeq();
            var data = Network.Packet.Encode(Network.MessageType.Request, route, seq, body);
            _ws.Send(data);
            _callbacks[seq] = onSuccess;
            if (onError != null)
                _errorCallbacks[seq] = onError;

            return seq;
        }

        public void Notify(string route, object body = null)
        {
            if (_ws == null || _ws.ReadyState != WebSocketState.Open)
                return;

            var data = Network.Packet.Encode(Network.MessageType.Notify, route, 0, body);
            _ws.Send(data);
        }

        public void On(string route, Action<object> handler)
        {
            if (!_eventHandlers.ContainsKey(route))
                _eventHandlers[route] = new List<Action<object>>();
            _eventHandlers[route].Add(handler);
        }

        public void Off(string route, Action<object> handler = null)
        {
            if (handler == null)
            {
                _eventHandlers.Remove(route);
            }
            else if (_eventHandlers.ContainsKey(route))
            {
                _eventHandlers[route].Remove(handler);
            }
        }

        public void RegisterRoute(string route, int id)
        {
            Network.RouteManager.RegisterRoute(route, id);
        }

        public bool IsConnected => _connected && _ws?.ReadyState == WebSocketState.Open;

        uint GetNextSeq()
        {
            _seq++;
            if (_seq == 0) _seq = 1;
            return _seq;
        }

        void HandlePacket(byte[] data)
        {
            try
            {
                var packet = Network.Packet.Decode(data);

                switch (packet.Type)
                {
                    case Network.MessageType.Response:
                        if (_callbacks.ContainsKey(packet.Seq))
                        {
                            var callback = _callbacks[packet.Seq];
                            _callbacks.Remove(packet.Seq);
                            _errorCallbacks.Remove(packet.Seq);
                            callback?.Invoke(packet.Body);
                        }
                        OnResponse?.Invoke(packet.Seq, packet.Body);
                        break;

                    case Network.MessageType.Notify:
                    case Network.MessageType.Error:
                        OnNotify?.Invoke(packet.Route, packet.Body);
                        if (_eventHandlers.ContainsKey(packet.Route))
                        {
                            foreach (var handler in _eventHandlers[packet.Route])
                            {
                                handler?.Invoke(packet.Body);
                            }
                        }
                        break;
                }
            }
            catch (Exception e)
            {
                Debug.LogError($"HandlePacket error: {e.Message}");
            }
        }

        void OnDestroy()
        {
            Disconnect();
        }
    }
}