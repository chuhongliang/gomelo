class_name GomeloClient
extends Node

signal connected()
signal disconnected()
signal error(message: String)
signal response(seq: int, body: Variant)
signal notify(route: String, body: Variant)

enum ProtocolType {
	WEBSOCKET,
	TCP,
	UDP
}

var host: String = "localhost"
var port: int = Protocol.DEFAULT_PORT
var protocol: ProtocolType = ProtocolType.WEBSOCKET
var timeout: int = Protocol.DEFAULT_TIMEOUT
var heartbeat_interval: int = 30000
var reconnect_interval: int = 3000
var max_reconnect_attempts: int = 5

var _websocket: WebSocketPeer
var _tcp_stream: StreamPeerTCP
var _udp_peer: PacketPeerUDP
var _connected := false
var _closed := false
var _pending: Dictionary = {}
var _seq := 0
var _next_route_id := 1
var _reconnect_attempts := 0
var _reconnect_timer: SceneTreeTimer
var _heartbeat_timer: SceneTreeTimer
var _event_handlers: Dictionary = {}
var _tcp_buffer: Array = []
var _udp_buffer: Array = []
var _schema_received := false
var _route_id_to_codec: Dictionary = {}
var _route_id_to_type_url: Dictionary = {}
var _protobuf_codec: RefCounted = null

func _ready() -> void:
	_websocket = WebSocketPeer.new()

func _process(delta: float) -> void:
	match protocol:
		ProtocolType.WEBSOCKET:
			_process_websocket()
		ProtocolType.TCP:
			_process_tcp()
		ProtocolType.UDP:
			_process_udp()

func _process_websocket() -> void:
	if _websocket.get_ready_state() == WebSocketPeer.STATE_OPEN:
		_websocket.poll()
		var packet := _websocket.get_packet()
		if packet.size() > 0:
			_handle_packet(packet)
	elif _websocket.get_ready_state() == WebSocketPeer.STATE_CLOSED:
		if _connected and not _closed:
			_connected = false
			disconnected.emit()
			_try_reconnect()

func _process_tcp() -> void:
	if _tcp_stream == null:
		return
	
	if _tcp_stream.get_status() == StreamPeerTCP.STATUS_CONNECTED:
		while _tcp_stream.get_available_bytes() > 0:
			var data := _tcp_stream.get_data(65536)
			if data[0] == OK:
				_tcp_buffer.append_array(data[1])
				_parse_tcp_buffer()
	elif _connected:
		_connected = false
		disconnected.emit()
		if not _closed:
			_try_reconnect()

func _process_udp() -> void:
	if _udp_peer == null:
		return
	
	while _udp_peer.get_available_packet_count() > 0:
		var packet := _udp_peer.get_packet()
		if packet.size() > 0:
			_handle_packet(packet)

func _parse_tcp_buffer() -> void:
	while _tcp_buffer.size() >= 4:
		var length := (_tcp_buffer[0] << 24) | (_tcp_buffer[1] << 16) | (_tcp_buffer[2] << 8) | _tcp_buffer[3]
		var total_len := 4 + length
		
		if _tcp_buffer.size() < total_len:
			break
		
		var packet_data: Array = []
		for i in range(4, total_len):
			packet_data.append(_tcp_buffer[i])
		_tcp_buffer = _tcp_buffer.slice(total_len)
		
		var packed := PackedByteArray()
		for b in packet_data:
			packed.append(b)
		_handle_packet(packed)

func connect_to_server(p_host: String = "", p_port: int = -1, p_protocol: ProtocolType = ProtocolType.WEBSOCKET) -> int:
	if not p_host.is_empty():
		host = p_host
	if p_port > 0:
		port = p_port
	protocol = p_protocol
	_closed = false
	_reconnect_attempts = 0

	match protocol:
		ProtocolType.TCP:
			return _connect_tcp()
		ProtocolType.UDP:
			return _connect_udp()
		ProtocolType.WEBSOCKET:
		default:
			return _connect_websocket()

func _connect_websocket() -> int:
	var url := "ws://%s:%d" % [host, port]
	var err := _websocket.connect_to_url(url)
	if err != OK:
		push_error("Failed to connect: %s" % err)
		if _on_error.size() > 0:
			_on_error[0].call("Failed to connect: %s" % err)
		return err
	
	_connected = true
	_emit_connected()
	_start_heartbeat()
	return OK

func _connect_tcp() -> int:
	_tcp_stream = StreamPeerTCP.new()
	var err := _tcp_stream.connect_to_host(host, port)
	if err != OK:
		push_error("Failed to connect TCP: %s" % err)
		if _on_error.size() > 0:
			_on_error[0].call("Failed to connect TCP: %s" % err)
		return err
	
	_connected = true
	_tcp_buffer.clear()
	_emit_connected()
	_start_heartbeat()
	return OK

func _connect_udp() -> int:
	_udp_peer = PacketPeerUDP.new()
	var err := _udp_peer.connect_to_host(host, port)
	if err != OK:
		push_error("Failed to connect UDP: %s" % err)
		if _on_error.size() > 0:
			_on_error[0].call("Failed to connect UDP: %s" % err)
		return err
	
	_connected = true
	_emit_connected()
	return OK

func disconnect_from_server() -> void:
	_closed = true
	_stop_heartbeat()
	_stop_reconnect()
	_connected = false

	match protocol:
		ProtocolType.WEBSOCKET:
			_websocket.close(1000, "Client disconnect")
		ProtocolType.TCP:
			if _tcp_stream != null:
				_tcp_stream.disconnect_from_host()
				_tcp_stream = null
		ProtocolType.UDP:
			if _udp_peer != null:
				_udp_peer.close()
				_udp_peer = null

	_clear_pending("Disconnected")

func request(route: String, body: Dictionary = {}) -> int:
	if not _connected:
		_error("Not connected")
		return -1

	var seq := _get_next_seq()
	var packet := Packet.encode(Protocol.MessageType.REQUEST, route, seq, body)
	_send_packet(packet)

	var timer := _create_timeout_timer(seq)
	_pending[seq] = {"resolve": null, "reject": null, "timer": timer}

	return seq

func request_with_callback(route: String, body: Dictionary, on_success: Callable, on_error: Callable = Callable()) -> void:
	if not _connected:
		if on_error.is_valid():
			on_error.call("Not connected")
		return

	var seq := _get_next_seq()
	var packet := Packet.encode(Protocol.MessageType.REQUEST, route, seq, body)
	_send_packet(packet)

	var timer := _create_timeout_timer(seq)
	_pending[seq] = {
		"resolve": on_success,
		"reject": on_error,
		"timer": timer
	}

func request_sync(route: String, body: Dictionary = {}) -> Variant:
	var result := []
	var error_msg := []
	var done := []

	request_with_callback(route, body,
		func(data): result.append(data); done.append(true),
		func(err): error_msg.append(err); done.append(true)
	)

	var timeout_sec := float(timeout) / 1000.0
	var elapsed := 0.0
	while done.is_empty() and elapsed < timeout_sec:
		await get_tree().process, "idleFrame"
		elapsed += get_process_delta_time()

	if not error_msg.is_empty():
		push_error(str(error_msg[0]))
		return null
	if done.is_empty():
		push_error("Request timeout")
		return null

	return result[0] if not result.is_empty() else null

func notify(route: String, body: Dictionary = {}) -> void:
	if not _connected:
		return

	var packet := Packet.encode(Protocol.MessageType.NOTIFY, route, 0, body)
	_send_packet(packet)

func _send_packet(packet: PackedByteArray) -> void:
	match protocol:
		ProtocolType.TCP:
			if _tcp_stream != null and _tcp_stream.get_status() == StreamPeerTCP.STATUS_CONNECTED:
				var length_bytes := PackedByteArray([(packet.size() >> 24) & 0xFF, (packet.size() >> 16) & 0xFF, (packet.size() >> 8) & 0xFF, packet.size() & 0xFF])
				_tcp_stream.put_data(length_bytes)
				_tcp_stream.put_data(packet)
		ProtocolType.UDP:
			if _udp_peer != null:
				_udp_peer.put_packet(packet)
		ProtocolType.WEBSOCKET:
			if _websocket.get_ready_state() == WebSocketPeer.STATE_OPEN:
				_websocket.send(packet)

func on(route: String, handler: Callable) -> void:
	if not _event_handlers.has(route):
		_event_handlers[route] = []
	_event_handlers[route].append(handler)

func off(route: String, handler: Callable = Callable()) -> void:
	if not _event_handlers.has(route):
		return

	if handler.is_valid():
		_event_handlers[route].erase(handler)
	else:
		_event_handlers[route].clear()

func off_all(route: String) -> void:
	_event_handlers.erase(route)

func emit_event(route: String, body: Variant) -> void:
	if _event_handlers.has(route):
		for handler in _event_handlers[route]:
			if handler.is_valid():
				handler.call(body)

func is_connected() -> bool:
	match protocol:
		ProtocolType.TCP:
			return _tcp_stream != null and _tcp_stream.get_status() == StreamPeerTCP.STATUS_CONNECTED
		ProtocolType.UDP:
			return _udp_peer != null
		ProtocolType.WEBSOCKET:
			return _websocket.get_ready_state() == WebSocketPeer.STATE_OPEN
	return false

func register_route(route: String, id: int) -> void:
	Packet.RouteManager.register_route(route, id)

func generate_route_id() -> int:
	var id := _next_route_id
	_next_route_id += 1
	return id

var _on_connected: Array = []
var _on_disconnected: Array = []
var _on_error: Array = []

func on_connected(handler: Callable) -> void:
	_on_connected.append(handler)

func on_disconnected(handler: Callable) -> void:
	_on_disconnected.append(handler)

func on_error(handler: Callable) -> void:
	_on_error.append(handler)

func _emit_connected() -> void:
	connected.emit()
	for handler in _on_connected:
		if handler.is_valid():
			handler.call()

func _start_heartbeat() -> void:
	if protocol == ProtocolType.UDP:
		return
	
	_stop_heartbeat()
	_heartbeat_timer = get_tree().create_timer(float(heartbeat_interval) / 1000.0)
	_heartbeat_timer.timeout.connect(_send_heartbeat)

func _stop_heartbeat() -> void:
	if _heartbeat_timer != null:
		_heartbeat_timer.time_left = 0
		_heartbeat_timer = null

func _send_heartbeat() -> void:
	if is_connected():
		notify("sys.heartbeat", {"ts": Time.get_unix_time_from_system() * 1000})
		_start_heartbeat()

func _try_reconnect() -> void:
	if _closed or protocol == ProtocolType.UDP or _reconnect_attempts >= max_reconnect_attempts:
		return

	_reconnect_attempts += 1
	var delay := float(_reconnect_attempts * reconnect_interval) / 1000.0

	_reconnect_timer = get_tree().create_timer(delay)
	_reconnect_timer.timeout.connect(_do_reconnect)

func _stop_reconnect() -> void:
	if _reconnect_timer != null:
		_reconnect_timer.time_left = 0
		_reconnect_timer = null

func _do_reconnect() -> void:
	if _closed:
		return

	_websocket.close(1001, "Reconnecting")

	var url := "ws://%s:%d" % [host, port]
	var err := _websocket.connect_to_url(url)

	if err == OK:
		_connected = true
		_start_heartbeat()
	else:
		_try_reconnect()

func _get_next_seq() -> int:
	_seq += 1
	if _seq > 0x7FFFFFFF:
		_seq = 1
	return _seq

func _create_timeout_timer(seq: int) -> SceneTreeTimer:
	var timer := get_tree().create_timer(float(timeout) / 1000.0)
	timer.timeout.connect(_on_request_timeout.bind(seq))
	return timer

func _on_request_timeout(seq: int) -> void:
	if _pending.has(seq):
		var pending := _pending[seq]
		_pending.erase(seq)
		if pending["reject"].is_valid():
			pending["reject"].call("Request timeout")

func _handle_packet(data: PackedByteArray) -> void:
	var packet := Packet.decode(data)

	if packet.is_schema:
		_handle_schema(packet.body)
		return

	match packet.type:
		Protocol.MessageType.RESPONSE:
			_response_handler(packet)
		Protocol.MessageType.NOTIFY, Protocol.MessageType.ERROR:
			_notify_handler(packet)
		Protocol.MessageType.REQUEST:
			push_warning("Received REQUEST, not handled in client mode")

func _handle_schema(data: Variant) -> void:
	if data == null or not data is Dictionary:
		return
	var schema_data = data.get("data")
	if schema_data == null or not schema_data is Dictionary:
		return
	var routes = schema_data.get("routes")
	if routes == null or not routes is Array:
		return

	for r in routes:
		if not r is Dictionary:
			continue
		var route_str: String = r.get("route")
		var id: int = r.get("id")
		var codec: String = r.get("codec")
		var type_url: String = r.get("typeUrl")

		if route_str.is_empty():
			continue

		Packet.RouteManager.register_route(route_str, id)
		if not codec.is_empty():
			_route_id_to_codec[id] = codec
			if not type_url.is_empty():
				_route_id_to_type_url[id] = type_url

	_schema_received = true

func _response_handler(packet: Packet) -> void:
	response.emit(packet.seq, packet.body)

	if _pending.has(packet.seq):
		var pending := _pending[packet.seq]
		_pending.erase(packet.seq)

		if pending["timer"] != null:
			pending["timer"].time_left = 0

		if packet.body != null and packet.body.has("error"):
			if pending["reject"].is_valid():
				pending["reject"].call(packet.body["error"])
		elif pending["resolve"].is_valid():
			pending["resolve"].call(packet.body)

func _notify_handler(packet: Packet) -> void:
	notify.emit(packet.route, packet.body)
	emit_event(packet.route, packet.body)

func _error(msg: String) -> void:
	error.emit(msg)
	for handler in _on_error:
		if handler.is_valid():
			handler.call(msg)

func _clear_pending(msg: String) -> void:
	for seq in _pending.keys():
		var pending := _pending[seq]
		if pending["reject"].is_valid():
			pending["reject"].call(msg)
	_pending.clear()