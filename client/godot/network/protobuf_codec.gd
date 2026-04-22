class_name ProtobufCodec
extends RefCounted

var _route_to_id := {}
var _id_to_route := {}
var _parsers := {}
var _next_id := 0

func register_route(route: String, id: int) -> void:
	_route_to_id[route] = id
	_id_to_route[id] = route

func register_message(route: String, id: int) -> void:
	register_route(route, id)

func get_route_id(route: String) -> int:
	return _route_to_id.get(route, 0)

func get_route(id: int) -> String:
	return _id_to_route.get(id, "")

func encode(msg_type: int, route: String, seq: int, body: Dictionary) -> PackedByteArray:
	var route_id := get_route_id(route) if msg_type == Protocol.MessageType.REQUEST else 0
	var body_json := JSON.stringify(body)
	var body_bytes := body_json.to_utf8_buffer()
	
	var header_size := 1 + 1 + 8
	if route_id > 0:
		header_size += 2
	else:
		header_size += route.to_utf8_buffer().size() + 1
	
	var total_size := header_size + body_bytes.size()
	var buffer := PackedByteArray()
	buffer.resize(total_size)
	
	var offset := 0
	buffer[offset] = msg_type
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
		var route_bytes := route.to_utf8_buffer()
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

func decode(data: PackedByteArray) -> Dictionary:
	if data.size() < 10:
		return {}
	
	var msg_type := data[0]
	var offset := 1
	var flag := data[offset]
	offset += 1
	
	var route := ""
	if flag == Protocol.RouteFlag.ROUTE_ID:
		if data.size() < offset + 2:
			return {}
		var route_id := (data[offset] << 8) | data[offset + 1]
		offset += 2
		route = get_route(route_id)
	else:
		var start := offset
		while offset < data.size() and data[offset] != 0:
			offset += 1
		route = data.slice(start, offset).get_string_from_utf8()
		offset += 1
	
	if data.size() < offset + 8:
		return {}
	
	var seq := 0
	for i in range(8):
		seq = (seq << 8) | data[offset + i]
	offset += 8
	
	var body_bytes := data.slice(offset) if offset < data.size() else PackedByteArray()
	var body_var := null
	if body_bytes.size() > 0:
		var json_str := body_bytes.get_string_from_utf8()
		if not json_str.is_empty():
			var json := JSON.new()
			if json.parse(json_str) == OK:
				body_var = json.data
	
	return {
		"type": msg_type,
		"route": route,
		"seq": seq,
		"body": body_var
	}