class_name GomeloClient
extends Node

signal connected()
signal disconnected()
signal error(message: String)
signal response(seq: int, body: Variant)
signal notify(route: String, body: Variant)

var host: String = "localhost"
var port: int = Protocol.DEFAULT_PORT
var timeout: int = Protocol.DEFAULT_TIMEOUT
var heartbeat_interval: int = 30000
var reconnect_interval: int = 3000
var max_reconnect_attempts: int = 5

var _websocket: WebSocketPeer
var _connected := false
var _closed := false
var _pending: Dictionary = {}
var _seq := 0
var _next_route_id := 1
var _reconnect_attempts := 0
var _reconnect_timer: SceneTreeTimer
var _heartbeat_timer: SceneTreeTimer
var _event_handlers: Dictionary = {}

func _ready() -> void:
	_websocket = WebSocketPeer.new()

func _process(delta: float) -> void:
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

func connect_to_server(host: String = "", port: int = -1) -> int:
	if not host.is_empty():
		self.host = host
	if port > 0:
		self.port = port
	_closed = false
	_reconnect_attempts = 0
	
	var url := "ws://%s:%d" % [self.host, self.port]
	
	var err := _websocket.connect_to_url(url)
	if err != OK:
		push_error("Failed to connect: %s" % err)
		if _on_error.size() > 0:
			_on_error[0].call("Failed to connect: %s" % err)
		return err
	
	_connected = true
	_connected.emit()
	_start_heartbeat()
	
	return OK

func disconnect_from_server() -> void:
	_closed = true
	_stop_heartbeat()
	_stop_reconnect()
	_websocket.close(1000, "Client disconnect")
	_connected = false
	_clear_pending("Disconnected")

func request(route: String, body: Dictionary = {}) -> int:
	if _websocket.get_ready_state() != WebSocketPeer.STATE_OPEN:
		_error("Not connected")
		return -1
	
	var seq := _get_next_seq()
	var packet := Packet.encode(Protocol.MessageType.REQUEST, route, seq, body)
	_websocket.send(packet)
	
	var timer := _create_timeout_timer(seq)
	_pending[seq] = {"resolve": null, "reject": null, "timer": timer}
	
	return seq

func request_with_callback(route: String, body: Dictionary, on_success: Callable, on_error: Callable = Callable()) -> void:
	if _websocket.get_ready_state() != WebSocketPeer.STATE_OPEN:
		if on_error.is_valid():
			on_error.call("Not connected")
		return
	
	var seq := _get_next_seq()
	var packet := Packet.encode(Protocol.MessageType.REQUEST, route, seq, body)
	_websocket.send(packet)
	
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
	if _websocket.get_ready_state() != WebSocketPeer.STATE_OPEN:
		return
	
	var packet := Packet.encode(Protocol.MessageType.NOTIFY, route, 0, body)
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
	return _websocket.get_ready_state() == WebSocketPeer.STATE_OPEN

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

func _start_heartbeat() -> void:
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
	if _closed or _reconnect_attempts >= max_reconnect_attempts:
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
	
	if err != OK:
		_try_reconnect()
	else:
		_connected = true
		_start_heartbeat()

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
	
	match packet.type:
		Protocol.MessageType.RESPONSE:
			_response_handler(packet)
		Protocol.MessageType.NOTIFY, Protocol.MessageType.ERROR:
			_notify_handler(packet)
		Protocol.MessageType.REQUEST:
			push_warning("Received REQUEST, not handled in client mode")

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
	if _on_error.is_empty():
		error.emit(msg)
	else:
		for handler in _on_error:
			if handler.is_valid():
				handler.call(msg)

func _clear_pending(msg: String) -> void:
	for seq in _pending.keys():
		var pending := _pending[seq]
		if pending["reject"].is_valid():
			pending["reject"].call(msg)
	_pending.clear()