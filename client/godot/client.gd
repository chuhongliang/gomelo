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

var _websocket: WebSocketPeer
var _connected := false
var _pending: Dictionary = {}
var _seq := 0
var _reconnect_delay := 1.0
var _reconnect_max := 30.0
var _should_reconnect := true

func _ready() -> void:
	_websocket = WebSocketPeer.new()

func _process(delta: float) -> void:
	if _websocket.get_ready_state() == WebSocketPeer.STATE_OPEN:
		_websocket.poll()
		var packet := _websocket.get_packet()
		if packet.size() > 0:
			_handle_packet(packet)

func connect_to_server(host: String = "", port: int = -1, auto_reconnect := true) -> int:
	if not host.is_empty():
		self.host = host
	if port > 0:
		self.port = port
	_should_reconnect = auto_reconnect
	
	var url := "ws://%s:%d" % [self.host, self.port]
	
	var err := _websocket.connect_to_url(url)
	if err != OK:
		push_error("Failed to connect: %s" % err)
		return err
	
	_connected = true
	connected.emit()
	return OK

func disconnect_from_server() -> void:
	_should_reconnect = false
	_websocket.close()
	_connected = false
	disconnected.emit()

func request(route: String, body: Dictionary = {}) -> int:
	if _websocket.get_ready_state() != WebSocketPeer.STATE_OPEN:
		error.emit("Not connected")
		return -1
	
	var seq := _get_next_seq()
	var packet := Packet.encode(Protocol.MessageType.REQUEST, route, seq, body)
	_websocket.send(packet)
	
	var timer := _create_timeout_timer(seq)
	_pending[seq] = {"resolve": null, "reject": null, "timer": timer}
	
	return seq

func request_with_callback(route: String, body: Dictionary, on_success: Callable, on_error: Callable = Callable()) -> void:
	if _websocket.get_ready_state() != WebSocketPeer.STATE_OPEN:
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

func notify(route: String, body: Dictionary = {}) -> void:
	if _websocket.get_ready_state() != WebSocketPeer.STATE_OPEN:
		return
	
	var packet := Packet.encode(Protocol.MessageType.NOTIFY, route, 0, body)
	_websocket.send(packet)

func on(route: String, handler: Callable) -> void:
	var key := "event_" + route
	if not has_user_signal(key):
		add_user_signal(key)
	connect(key, handler)

func off(route: String, handler: Callable = Callable()) -> void:
	var key := "event_" + route
	if handler.is_valid():
		disconnect(key, handler)
	else:
		set(key, null)

func emit_event(route: String, body: Variant) -> void:
	var key := "event_" + route
	if has_signal(key):
		emit_signal(key, body)

func is_connected() -> bool:
	return _websocket.get_ready_state() == WebSocketPeer.STATE_OPEN

func register_route(route: String, id: int) -> void:
	Packet.RouteManager.register_route(route, id)

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