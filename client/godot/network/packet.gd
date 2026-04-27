class_name Packet
extends RefCounted

var type: int
var route: String
var seq: int
var body: Variant
var is_schema: bool = false

func _init(t: int = 0, r: String = "", s: int = 0, b: Variant = null):
	type = t
	route = r
	seq = s
	body = b

static func encode(t: int, route: String, seq: int, body: Dictionary) -> PackedByteArray:
	var packet := Packet.new(t, route, seq, body)
	return packet._to_bytes()

static func decode(data: PackedByteArray) -> Packet:
	return Packet.new()._from_bytes(data)

static func decode_with_schema(data: PackedByteArray, codec: RefCounted) -> Packet:
	var packet := Packet.new()._from_bytes(data)
	if packet.is_schema:
		return packet
	if packet.body != null:
		packet.body = codec.decode_body(packet.route, packet.body)
	return packet

func _to_bytes() -> PackedByteArray:
	var json_str := JSON.stringify(body)
	var body_bytes := json_str.to_utf8_buffer()
	
	var route_id := RouteManager.get_route_id(route)
	var route_bytes := route.to_utf8_buffer()
	
	var header_size := 1 + 1 + 8
	var route_part_size: int
	
	if route_id > 0:
		route_part_size = 3
		header_size += 2
	else:
		route_part_size = route_bytes.size() + 1
		header_size += route_bytes.size() + 1
	
	var total_size := header_size + body_bytes.size()
	var buffer := PackedByteArray()
	buffer.resize(total_size)
	
	var offset := 0
	buffer[offset] = type
	offset += 1
	
	if route_id > 0:
		buffer[offset] = Protocol.RouteFlag.ROUTE_ID
		offset += 1
		buffer[offset] = (route_id >> 8) & 0xFF
		buffer[offset + 1] = route_id & 0xFF
		offset += 2
	else:
		buffer[offset] = Protocol.RouteFlag.ROUTE_STRING
		offset += 1
		for i in route_bytes.size():
			buffer[offset + i] = route_bytes[i]
		offset += route_bytes.size()
		buffer[offset] = 0
		offset += 1
	
	buffer[offset] = (seq >> 56) & 0xFF
	buffer[offset + 1] = (seq >> 48) & 0xFF
	buffer[offset + 2] = (seq >> 40) & 0xFF
	buffer[offset + 3] = (seq >> 32) & 0xFF
	buffer[offset + 4] = (seq >> 24) & 0xFF
	buffer[offset + 5] = (seq >> 16) & 0xFF
	buffer[offset + 6] = (seq >> 8) & 0xFF
	buffer[offset + 7] = seq & 0xFF
	offset += 8
	
	for i in body_bytes.size():
		buffer[offset + i] = body_bytes[i]
	
	return buffer

func _from_bytes(data: PackedByteArray) -> Packet:
	if data.size() < 10:
		push_error("Packet too short")
		return self
	
	type = data[0]
	var offset := 1
	
	var flag := data[offset]
	offset += 1
	
	route = ""
	if flag == Protocol.RouteFlag.ROUTE_ID:
		if data.size() < offset + 2:
			push_error("Invalid route id")
			return self
		var route_id := (data[offset] << 8) | data[offset + 1]
		offset += 2
		route = RouteManager.get_route(route_id)
	else:
		var start := offset
		while offset < data.size() and data[offset] != 0:
			offset += 1
		route = data.slice(start, offset).get_string_from_utf8()
		offset += 1
	
	if data.size() < offset + 8:
		push_error("Invalid seq")
		return self
	
	seq = 0
	for i in range(8):
		seq = (seq << 8) | data[offset + i]
	offset += 8
	
	if offset < data.size():
		var body_bytes := data.slice(offset)
		var json_str := body_bytes.get_string_from_utf8()
		if json_str.is_empty():
			body = null
		else:
			var json := JSON.new()
			if json.parse(json_str) == OK:
				if json.data.has("type") and json.data.get("type") == "schema":
					is_schema = true
				body = json.data
			else:
				body = body_bytes
	else:
		body = null
	
	return self


class RouteManager:
	static var _route_to_id := {}
	static var _id_to_route := {}
	static var _next_id := 0
	
	static func register_route(route: String, id: int) -> void:
		_route_to_id[route] = id
		_id_to_route[id] = route
	
	static func get_route_id(route: String) -> int:
		return _route_to_id.get(route, 0)
	
	static func get_route(id: int) -> String:
		return _id_to_route.get(id, "")
	
	static func clear() -> void:
		_route_to_id.clear()
		_id_to_route.clear()
		_next_id = 0